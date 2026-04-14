# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Project scaffold: Go module (`github.com/astromechza/aurelianprm`), directory structure, `CLAUDE.md`, `.golangci.yml`
- `internal/config`: JSON config loading with strict unknown-field rejection
- `migrations/00001_initial_schema.sql`: full SQLite schema — `blurry_dates`, `blurry_locations`, `people`, `contact_methods`, `addresses`, `pets`, `work`, `interactions`, `relationships`
- `internal/db`: SQLite connection with WAL mode, foreign-key enforcement, busy-timeout, and Goose migration runner
- `cmd/aurelianprm`: CLI entry point with `--version`, `hash-password` (bcrypt cost 12), and stubs for `serve` and `send-test-email`
