MODULE   := github.com/agent-jit/agentjit
BIN      := aj
CMD      := ./cmd/agentjit
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

.PHONY: build test lint clean install release

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

# Release: tag, update CHANGELOG.md, commit, push, and run goreleaser.
# Usage: make release V=0.2.0
release:
ifndef V
	$(error Usage: make release V=0.2.0)
endif
	@echo "=== Releasing v$(V) ==="
	@# 1. Ensure working tree is clean
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: working tree is not clean. Commit or stash changes first."; \
		exit 1; \
	fi
	@# 2. Update CHANGELOG.md: replace [Unreleased] header and add comparison link
	@if grep -q '^\#\# \[Unreleased\]' CHANGELOG.md 2>/dev/null; then \
		sed -i '' "s/^## \[Unreleased\]/## [$(V)] - $$(date +%Y-%m-%d)/" CHANGELOG.md; \
		PREV=$$(git describe --tags --abbrev=0 2>/dev/null); \
		if [ -n "$$PREV" ]; then \
			sed -i '' "s|\[Unreleased\]:.*|[$(V)]: https://github.com/agent-jit/AgentJIT/compare/$$PREV...v$(V)\n[Unreleased]: https://github.com/agent-jit/AgentJIT/compare/v$(V)...HEAD|" CHANGELOG.md; \
		fi; \
		git add CHANGELOG.md; \
		git commit -m "docs: update CHANGELOG for v$(V)"; \
	fi
	@# 3. Tag and push
	git tag v$(V)
	git push origin main v$(V)
	@# 4. Run goreleaser
	goreleaser release --clean
	@echo "=== v$(V) released ==="
