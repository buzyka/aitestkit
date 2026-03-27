.PHONY: test coverage lint fmt check

fmt:
	go fmt ./...

test:
	go test ./...

coverage:
	go test ./... -covermode=atomic -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

check: fmt test lint
