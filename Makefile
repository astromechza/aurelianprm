.PHONY: fmt test lint tidy verify

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

test:
	go test -race ./...

lint:
	golangci-lint run ./...

verify: fmt tidy test lint
