PREFIX ?= /opt/homebrew
BINDIR := $(PREFIX)/bin
SHAREDIR := $(PREFIX)/share/codemob

.PHONY: build install uninstall test clean

build:
	@echo "Building codemob..."
	@go build -o codemob .
	@echo "  → ./codemob"

install: build
	@echo "Installing to $(PREFIX)..."
	@mkdir -p $(BINDIR) $(SHAREDIR)
	@cp codemob $(BINDIR)/codemob
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
	@go test ./internal/mob/ -count=1 -v

clean:
	@rm -f codemob
	@echo "  Cleaned build artifacts."
