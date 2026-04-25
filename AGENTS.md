# aurelianprm — Agent Instructions

## Verification — MANDATORY

**MUST run `make verify` before every commit. No exceptions.**

`make verify` runs the exact same checks as CI:

1. `gofmt -l -w .` — format all Go files
2. `go mod tidy` — keep go.mod/go.sum clean
3. `go test -race ./...` — all tests with race detector
4. `golangci-lint run ./...` — lint with project config (`.golangci.yml`)

All four must pass with zero errors before `git commit`. Fix all failures first. A commit that skips `make verify` will break CI — do not do it.

## Development Rules

- Module: `github.com/astromechza/aurelianprm`
- Go version: see `go.mod`
- No new dependencies without explicit approval
- Keep `go.mod` tidy — run `go mod tidy` after any import changes
- Error handling: explicit, never swallow errors
- Use `run()` pattern in main — keep `main()` minimal

## Code Style

- Follow standard Go idioms
- No comments explaining WHAT — only WHY when non-obvious
- Use `github.com/stretchr/testify` for tests
- No inline CSS if frontend is added
