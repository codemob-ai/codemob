VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/codemob-ai/codemob/cmd.Version=$(VERSION)"

PREFIX ?= /opt/homebrew
BINDIR := $(PREFIX)/bin
SHAREDIR := $(PREFIX)/share/codemob

.PHONY: build install uninstall test clean release-dry-run

SESSION_QUEUE_TESTS := TestQueueUnknownAction|TestQueueSwitchRequiresTarget|TestQueueRequiresSession|TestClearQueueRequiresSession|TestInfoDoesNotClearQueuedAction|TestClearQueueRemovesQueuedAction|TestShellCdClearsQueuedActionForTargetMob|TestShellCdRootClearsQueuedActionForCurrentSession|TestShellClaudeClearsQueuedActionForCurrentMob|TestQueueIsolationBySession

build:
	@echo "Building codemob $(VERSION)..."
	@go build $(LDFLAGS) -o codemob .
	@echo "  → ./codemob"

install:
	@command -v go >/dev/null 2>&1 || { echo "Error: Go is not installed."; echo ""; echo "Install via Homebrew:  brew install go"; echo "Or visit:             https://go.dev/dl/"; exit 1; }
	@echo "Dev install — emulating Homebrew layout at $(PREFIX)"
	@echo ""
	@mkdir -p $(BINDIR) $(SHAREDIR)
	@echo "Building codemob $(VERSION)..."
	@go build $(LDFLAGS) -o $(BINDIR)/codemob .
	@cp codemob-shell.sh $(SHAREDIR)/codemob-shell.sh
	@echo "  → $(BINDIR)/codemob"
	@echo "  → $(SHAREDIR)/codemob-shell.sh"
	@echo ""
	@echo "Run 'codemob init' to set up shell integration."

uninstall:
	@echo "Removing codemob from $(PREFIX)..."
	@rm -f $(BINDIR)/codemob
	@rm -rf $(SHAREDIR)
	@echo "  Done."

test:
	@go test ./... -count=1 -v

test-session-queue:
	@go test ./internal/mob -run '$(SESSION_QUEUE_TESTS)' -count=1 -v

test-branch: test-session-queue

clean:
	@rm -f codemob
	@rm -rf dist
	@echo "  Cleaned build artifacts."

release-dry-run:
	@goreleaser release --snapshot --clean
