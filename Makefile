.PHONY: build test lint all clean

all: lint test build

build:
	go build -o codelens ./cmd/codelens

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f codelens
