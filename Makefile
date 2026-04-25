.PHONY: fmt test lint tidy verify install-hooks

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

test:
	go test -race ./...

lint:
	golangci-lint run ./...

verify: fmt tidy test lint

install-hooks:
	@printf '#!/bin/sh\nset -e\necho "pre-commit: running make verify..."\nmake verify\n' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook installed"
