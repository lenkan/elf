BINDIR ?= $(HOME)/.local/bin
COMPLETIONDIR ?= $(or $(XDG_DATA_HOME),$(HOME)/.local/share)/bash-completion/completions

.PHONY: install
install:
	mkdir -p "$(BINDIR)" "$(COMPLETIONDIR)"
	go build -o "$(BINDIR)/elf" .
	"$(BINDIR)/elf" completion bash >"$(COMPLETIONDIR)/elf"
