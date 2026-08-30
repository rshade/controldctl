.PHONY: build test lint fmt tidy

BINARY := controldctl

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

lint:
	go vet ./...
	gofmt -l .

fmt:
	gofmt -w .

tidy:
	go mod tidy
