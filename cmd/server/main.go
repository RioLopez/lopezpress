package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lopezpress/internal/clientip"
	"lopezpress/internal/config"
	"lopezpress/internal/handlers"
	"lopezpress/internal/middleware"
	"lopezpress/internal/router"
	"lopezpress/internal/services"
)

// main entry point.
func main() {
	// Offline recovery flag (spec #22 / TODO P1-17). Parsed before anything
	// else so a reset run never starts the HTTP server.
	resetRoot := flag.Bool("reset-root-password", false,
		"rotate the unique root account's password, print it once to stderr, and exit (offline recovery)")
	flag.Parse()

	loadDotEnv()
	cfg := config.Load()

	// Enforce security-sensitive config (JWT_SECRET strength in prod) before
	// opening the database, so a bad production setup fails fast.
	if err := cfg.ValidateSecurity(); err != nil {
		log.Fatalf("security config invalid: %v", err)
	}

	// Trusted-proxy list for client-IP resolution (spec 23 / P2-6). Parsed
	// before anything else so a typo in TRUSTED_PROXIES fails at startup
	// rather than silently degrading every IP-based decision to the proxy's
	// address.
	if err := clientip.Init(cfg.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}

	if *resetRoot {
		os.Exit(runResetRootPassword(cfg))
	}

	db, seededAdminPwd, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	// Print a console banner to stderr when a fresh admin account was seeded
	// (never written to log files). The banner tells the operator how to
	// change the password to a value of their choosing.
	if seededAdminPwd != "" {
		const banner = `
==================================================================
  FIRST BOOT — the root account was created.
  Username: admin   (role: root, the only super-admin)
  Password: %s
==================================================================

  This password was generated at startup and shown ONCE.
  - For local dev: use it to log in, then go to the admin page and
    change it to a memorable password of your choosing.
  - For production: you MUST change this password immediately.
    (Set the ADMIN_INITIAL_PASSWORD env var to pin a known value
     on first boot if you prefer.)
  - Forgotten later? Run: lopezpress -reset-root-password
    (admin cannot reset root by design; see spec #22)
==================================================================
`
		fmt.Fprintf(os.Stderr, banner, seededAdminPwd)
	}

	// Build the services layer (all business logic) and inject the shared
	// dependencies into the thin handlers package.
	svc := services.New(db, cfg)
	handlers.Init(db, cfg, svc)

	// Spec 29: posts created before the slug column existed get their
	// permanent link here (idempotent, one indexed query per boot).
	if n, err := svc.BackfillSlugs(context.Background()); err != nil {
		log.Printf("WARN slug backfill failed: %v", err)
	} else if n > 0 {
		log.Printf("slug backfill: %d post(s) received a permanent link", n)
	}

	// Spec 34: daily snapshot. Runs on boot and then on a coarse timer; every
	// check costs one os.Stat once today's file exists, so a 30-minute tick is
	// free compared to the request traffic around it.
	runAutoBackup := func() {
		if name, err := svc.MaybeAutoBackup(context.Background()); err != nil {
			log.Printf("WARN auto backup failed: %v", err)
		} else if name != "" {
			log.Printf("auto backup: wrote %s", name)
		}
	}
	runAutoBackup()
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for range t.C {
			runAutoBackup()
		}
	}()

	// Build the per-identity rate limiter from the loaded config.
	limiter := middleware.NewLimiter(middleware.LimiterConfig{
		RPS:   cfg.RateLimitRPS,
		Burst: cfg.RateLimitBurst,
	})

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Login-specific limiter (spec #3): IP + account dual bucket + 5-failure
	// lockout, applied only to /api/auth/login. The janitor reaps stale
	// lockout entries every minute and stops with the process.
	loginLimiter := middleware.NewLoginLimiter()
	loginLimiter.Start(ctx)

	h := router.New(cfg, limiter, loginLimiter)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h,
	}

	go func() {
		log.Printf("lopezpress listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server exited: %v", err)
		}
	}()

	// Posts have only two visibilities — public ("published") and
	// private ("private"). Scheduled publishing and drafts were removed
	// (spec 22), so there is no background scheduler to run; a post's
	// visibility is set at publish time and can be toggled in content
	// management.
	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}

// runResetRootPassword implements the `-reset-root-password` flag (spec #22).
//
// It exists because the HTTP API intentionally has no path back into root: an
// admin may not reset root (P0-1 fix) and root's own password change requires
// the current password, so a forgotten root password used to mean "edit the
// database by hand or lose the site". This makes that operation correct
// (bcrypt, forced rotation, session revocation) for whoever already has
// filesystem access — which is the same trust boundary as editing the DB.
//
// Returns the process exit code.
func runResetRootPassword(cfg *config.Config) int {
	// Refuse to touch a missing database file. InitDB would happily create and
	// seed a brand-new one, so a typo in DB_PATH would silently "succeed" on the
	// wrong database while the real site stays locked out.
	if _, err := os.Stat(cfg.DBPath); err != nil {
		fmt.Fprintf(os.Stderr,
			"refusing to reset: database %q is not readable (%v)\n"+
				"hint: run from the project root so .env is loaded, or set DB_PATH explicitly.\n",
			cfg.DBPath, err)
		return 1
	}

	db, seededPwd, err := config.InitDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database %q: %v\n", cfg.DBPath, err)
		return 1
	}

	password := seededPwd
	if password == "" {
		password, err = config.ResetRootPasswordForCLI(db)
		if err != nil {
			if errors.Is(err, config.ErrNoRootAccount) {
				fmt.Fprintln(os.Stderr,
					"refusing to reset: no root account in this database; restore a backup instead.")
			} else {
				fmt.Fprintf(os.Stderr, "failed to reset root password: %v\n", err)
			}
			return 1
		}
	}

	const banner = `
==================================================================
  ROOT PASSWORD RESET
  Username: admin   (role: root)
  Password: %s
==================================================================

  Shown ONCE, never written to a log file.
  - Every JWT issued before this reset is now invalid.
  - The next login is forced to /change-password.
==================================================================
`
	fmt.Fprintf(os.Stderr, banner, password)
	return 0
}

// loadDotEnv reads a .env file in the current directory, if present, and
// sets any variables that are not already set in the environment. This keeps
// the dependency-free config loader (internal/config) working with a standard
// .env file without pulling in a third-party env library.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		key, val, ok := splitKeyVal(line)
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func splitKeyVal(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			return line[:i], trimSpace(line[i+1:]), true
		}
	}
	return "", "", false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
