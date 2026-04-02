MODULE   := github.com/anthropics/agentjit
BIN      := aj
CMD      := ./cmd/agentjit
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

.PHONY: build test lint clean install

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

test:
	CGO_ENABLED=0 go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN)

install: build
	mv $(BIN) /usr/local/bin/$(BIN)
