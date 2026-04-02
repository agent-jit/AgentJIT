MODULE   := github.com/anthropics/agentjit
BIN      := aj
CMD      := ./cmd/agentjit

.PHONY: build test lint clean install

build:
	CGO_ENABLED=0 go build -o $(BIN) $(CMD)

test:
	CGO_ENABLED=0 go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN)

install: build
	mv $(BIN) /usr/local/bin/$(BIN)
