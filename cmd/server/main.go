package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"echo-content-api/internal/config"
	"echo-content-api/internal/handlers"
	"echo-content-api/internal/middleware"
	"echo-content-api/internal/router"
	"echo-content-api/internal/services"
)

// main entry point.
func main() {
	loadDotEnv()
	cfg := config.Load()

	// Enforce security-sensitive config (JWT_SECRET strength in prod) before
	// opening the database, so a bad production setup fails fast.
	if err := cfg.ValidateSecurity(); err != nil {
		log.Fatalf("security config invalid: %v", err)
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
  FIRST BOOT — a new admin account was created.
  Username: admin
  Password: %s
==================================================================

  This password was generated at startup and shown ONCE.
  - For local dev: use it to log in, then go to the admin page and
    change it to a memorable password of your choosing.
  - For production: you MUST change this password immediately.
    (Set the ADMIN_INITIAL_PASSWORD env var to pin a known value
     on first boot if you prefer.)
==================================================================
`
		fmt.Fprintf(os.Stderr, banner, seededAdminPwd)
	}

	// Build the services layer (all business logic) and inject the shared
	// dependencies into the thin handlers package.
	svc := services.New(db, cfg)
	handlers.Init(db, cfg, svc)

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
		log.Printf("echo-content-api listening on :%s", cfg.Port)
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
