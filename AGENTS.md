# aurelianprm — Agent Instructions

## Verification

Run `make verify` before claiming any task complete. It runs:

1. `gofmt -l -w .` — format all Go files
2. `go mod tidy` — keep go.mod/go.sum clean
3. `go test -race ./...` — all tests with race detector
4. `golangci-lint run ./...` — lint with project config

All four must pass. Fix failures before finishing.

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
