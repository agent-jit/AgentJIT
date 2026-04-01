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

install:
	go install $(CMD)
