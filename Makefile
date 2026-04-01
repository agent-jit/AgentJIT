MODULE   := github.com/anthropics/agentjit
BIN      := agentjit
CMD      := ./cmd/agentjit

.PHONY: build test lint clean install

build:
	go build -o $(BIN) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN)

install: build
	mv $(BIN) /usr/local/bin/$(BIN)
