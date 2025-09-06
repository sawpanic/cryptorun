.PHONY: build lint test
build:
	cd src && go build ./cmd/cprotocol
lint:
	golangci-lint run ./...
test:
	cd src && go test ./...
