BINDIR ?= $(HOME)/.local/bin
COMPLETIONDIR ?= $(or $(XDG_DATA_HOME),$(HOME)/.local/share)/bash-completion/completions

# Installs a development build as elf-dev, next to a released elf
.PHONY: install
install:
	mkdir -p "$(BINDIR)" "$(COMPLETIONDIR)"
	go build -o "$(BINDIR)/elf-dev" .
	"$(BINDIR)/elf-dev" completion bash | sed 's/\belf\b/elf-dev/g' >"$(COMPLETIONDIR)/elf-dev"
