.PHONY: build test clean install

BINARY := maestron
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)

install:
	go install $(LDFLAGS) .
