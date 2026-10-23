package services

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func smtpSend(ctx context.Context, sm smtpConfig, from, to, subject, body string) error {
	addr := sm.addr()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Implicit TLS (port 465) wraps the socket immediately.
	if sm.Port == 465 {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: sm.Host})
		if err := tlsConn.Handshake(); err != nil {
			return fmt.Errorf("tls handshake: %w", err)
		}
		conn = tlsConn
	}

	if err := expectOK(conn, "220"); err != nil {
		return fmt.Errorf("banner: %w", err)
	}

	// EHLO once on the (possibly TLS) connection.
	if err := smtpCmd(conn, "EHLO "+hostnameFor(sm.Host), "250"); err != nil {
		return err
	}

	// STARTTLS upgrade for 587 / non-465 servers that advertise it. After
	// upgrading we must re-EHLO. If the server does not support STARTTLS
	// (e.g. a local relay on port 25), fall back to plain.
	if sm.Port != 465 {
		startErr := smtpCmd(conn, "STARTTLS", "220")
		if startErr == nil {
			tlsConn := tls.Client(conn, &tls.Config{ServerName: sm.Host})
			if err := tlsConn.Handshake(); err != nil {
				return fmt.Errorf("starttls handshake: %w", err)
			}
			conn = tlsConn
			if err := smtpCmd(conn, "EHLO "+hostnameFor(sm.Host), "250"); err != nil {
				return err
			}
		}
		// else: no STARTTLS support; continue plain.
	}

	// AUTH when a username is provided.
	if sm.User != "" {
		auth := "AUTH LOGIN " + b64(sm.User) + " " + b64(sm.Pass)
		if err := smtpCmd(conn, auth, "235"); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	// MAIL / RCPT / DATA.
	if err := smtpCmd(conn, "MAIL FROM: <"+from+">", "250"); err != nil {
		return err
	}
	if err := smtpCmd(conn, "RCPT TO: <"+to+">", "250"); err != nil {
		return err
	}
	if err := smtpCmd(conn, "DATA", "354"); err != nil {
		return err
	}

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n" +
		"\r\n" + body + "\r\n.\r\n"

	if _, err := io.WriteString(conn, msg); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	if err := expectOK(conn, "250"); err != nil {
		return fmt.Errorf("message not accepted: %w", err)
	}
	_ = smtpCmd(conn, "QUIT", "221")
	return nil
}

// expectOK reads a line and checks the 3-digit code prefix matches.
func expectOK(conn net.Conn, want string) error {
	line, err := readSMTPLine(conn)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, want) {
		return fmt.Errorf("smtp: got %q, want prefix %q", line, want)
	}
	return nil
}

// smtpCmd sends a command and verifies the expected code prefix.
func smtpCmd(conn net.Conn, cmd, want string) error {
	if _, err := fmt.Fprintf(conn, "%s\r\n", cmd); err != nil {
		return err
	}
	return expectOK(conn, want)
}

// readSMTPLine reads one SMTP response, consuming multi-line "NNN-"
// continuations until the "NNN " final line.
func readSMTPLine(conn net.Conn) (string, error) {
	sc := bufio.NewScanner(conn)
	// SMTP lines are short; the default 64KB buffer is ample.
	line, err := scanLine(sc)
	if err != nil {
		return "", err
	}
	if len(line) >= 4 && line[3] == '-' {
		for {
			next, err := scanLine(sc)
			if err != nil {
				return "", err
			}
			if len(next) >= 4 && next[3] == ' ' {
				line = next
				break
			}
		}
	}
	return line, nil
}

// scanLine reads one line from a scanner, translating Scanner.Err to a
// meaningful value.
func scanLine(sc *bufio.Scanner) (string, error) {
	if !sc.Scan() {
		err := sc.Err()
		if err == nil {
			err = io.EOF
		}
		return "", err
	}
	return sc.Text(), nil
}

// b64 base64-encodes for AUTH LOGIN.
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// hostnameFor picks a sensible EHLO hostname; providers usually accept any.
func hostnameFor(host string) string {
	if h := strings.TrimSpace(host); h != "" {
		// Strip port if the host string carries one (we only ever pass host).
		if i := strings.LastIndex(h, ":"); i > 0 {
			h = h[:i]
		}
		if h != "" {
			return h
		}
	}
	return "localhost"
}
