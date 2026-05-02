// Package main is the entrypoint for aurelianprm.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/digest"
	"github.com/astromechza/aurelianprm/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "send-digest" {
		return runDigest()
	}
	return runServe()
}

func runServe() error {
	dbPath := flag.String("db", "aurelianprm.db", "path to SQLite database file")
	addr := flag.String("addr", "", "HTTP listen address (overrides PORT env var)")
	flag.Parse()

	listenAddr := *addr
	if listenAddr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		listenAddr = ":" + port
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck

	srv, err := web.NewServer(dal.New(sqlDB, *dbPath))
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	fmt.Fprintf(os.Stderr, "listening on %s\n", listenAddr)
	return http.ListenAndServe(listenAddr, srv.Handler())
}

func runDigest() error {
	fs := flag.NewFlagSet("send-digest", flag.ContinueOnError)
	dbPath := fs.String("db", "aurelianprm.db", "path to SQLite database file")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	cfg, err := digestConfigFromEnv()
	if err != nil {
		return err
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck

	return digest.SendDigest(context.Background(), dal.New(sqlDB, *dbPath), cfg)
}

// digestConfigFromEnv reads SMTP and recipient config from environment variables.
// Returns an error listing all missing required variables.
func digestConfigFromEnv() (digest.Config, error) {
	var missing []string
	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	port := 587
	if v := os.Getenv("SMTP_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return digest.Config{}, fmt.Errorf("SMTP_PORT must be an integer: %w", err)
		}
		port = p
	}

	cfg := digest.Config{
		SMTPHost: require("SMTP_HOST"),
		SMTPPort: port,
		SMTPUser: require("SMTP_USER"),
		SMTPPass: require("SMTP_PASS"),
		SMTPFrom: require("SMTP_FROM"),
		SMTPSSL:  os.Getenv("SMTP_SSL") == "true",
		DigestTo: require("DIGEST_EMAIL_ADDRESS"),
	}
	if len(missing) > 0 {
		return digest.Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}
