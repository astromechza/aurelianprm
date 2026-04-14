# AurelianPRM — Claude Code Instructions

## Project Overview

Personal Relationship Manager (PRM). Single Go binary, SQLite, HTMX frontend.
Spec: `docs/superpowers/specs/` — read this before making significant changes.

## Module

`github.com/astromechza/aurelianprm`

## Key Commands

- Run server: `go run ./cmd/aurelianprm serve --config ./config.json`
- Run tests:  `go test -race -cover ./...`
- Lint:       `golangci-lint run`
- Build:      `go build ./...`
- Release build: `go build -ldflags "-X main.version=$(git describe --tags)" ./cmd/aurelianprm`

## Architecture Rules

- All handlers in `internal/web/` must check `HX-Request` header and return fragment or full page accordingly
- No business logic in handlers — call `internal/store` directly
- No inline CSS. No custom CSS beyond CSS variable overrides in `static/css/custom.css`
- All `blurry_dates` and `blurry_locations` are referenced by FK — never embed fields inline
- Relationship `to_type` must be one of: `person`, `address`, `pet`, `work`
- Config is JSON, loaded from path passed via `--config` flag
- Passwords stored as bcrypt hashes only (cost 12)

## Development Workflow

- Follow the phased implementation plan
- All phases must pass `go build`, `golangci-lint run`, and `go test -race -cover ./...` before marking complete
- If the spec needs updating due to implementation findings, update `docs/superpowers/specs/` and commit alongside the code change
- Templates are parsed eagerly at startup via `template.Must` — a unit test also parses all templates to catch errors in CI

## Testing

- Use `github.com/stretchr/testify`
- Store tests use a real in-memory SQLite instance (not mocked)
- Reminder scheduler uses an injected clock interface for deterministic tests
- HTTP handler tests use `net/http/httptest`
- Target 90–100% branch coverage
