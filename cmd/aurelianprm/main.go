// Command aurelianprm is the AurelianPRM personal relationship manager server.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/db"
)

// version is set at build time via -ldflags "-X main.version=v0.1.0".
// It defaults to "dev" for local builds.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "--version", "-version", "version":
		fmt.Println(buildVersion())
		return nil
	case "--help", "-help", "-h", "help":
		printUsage()
		return nil
	case "serve":
		return runServe(args[1:])
	case "hash-password":
		return runHashPassword()
	case "send-test-email":
		return runSendTestEmail(args[1:])
	default:
		return fmt.Errorf("unknown command %q — run with --help for usage", args[0])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `aurelianprm — Personal Relationship Manager

Usage:
  aurelianprm <command> [flags]

Commands:
  serve            Start the HTTP server
  hash-password    Generate a bcrypt hash of a password (cost 12)
  send-test-email  Send a test reminder email via configured SMTP
  version          Print version information

Flags:
  --version    Print version and exit
  --help       Show this help`)
}

// buildVersion returns the version string including VCS commit info.
func buildVersion() string {
	commit := "unknown"
	built := "unknown"

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				c := s.Value
				if len(c) > 7 {
					c = c[:7]
				}
				commit = c
			case "vcs.modified":
				if s.Value == "true" {
					commit += "-dirty"
				}
			case "vcs.time":
				built = s.Value
			}
		}
	}

	return fmt.Sprintf("aurelianprm %s (commit: %s, built: %s)", version, commit, built)
}

// runServe starts the HTTP server.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to JSON config file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("--config flag is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close() //nolint:errcheck

	// TODO: Phase 3 — wire up HTTP server.
	return errors.New("serve not yet implemented (phase 3)")
}

// runHashPassword reads a password interactively and prints its bcrypt hash.
func runHashPassword() error {
	fmt.Fprint(os.Stderr, "Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}

	if len(password) == 0 {
		return errors.New("password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword(password, 12)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	fmt.Println(string(hash))
	return nil
}

// runSendTestEmail sends a test email using the configured SMTP settings.
func runSendTestEmail(args []string) error {
	fs := flag.NewFlagSet("send-test-email", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to JSON config file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("--config flag is required")
	}

	_, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	// TODO: Phase 5 — implement SMTP email sending.
	return errors.New("send-test-email not yet implemented (phase 5)")
}
