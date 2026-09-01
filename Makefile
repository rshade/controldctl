.PHONY: build test lint fmt tidy race ci

BINARY := controldctl

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

lint:
	go vet ./...
	@fmt_files=$$(gofmt -l .); \
	if [ -n "$$fmt_files" ]; then \
		echo "$$fmt_files"; \
		exit 1; \
	fi

fmt:
	gofmt -w .

tidy:
	go mod tidy

race:
	go test -race ./...

ci: build lint race
