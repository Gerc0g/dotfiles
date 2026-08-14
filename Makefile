SHELL := /bin/bash

BASH_FILES := bootstrap.sh $(wildcard scripts/*.sh) \
	$(wildcard skills-stash/wiki/scripts/*.sh) \
	$(wildcard skills-stash/wiki/hooks/*.sh)
ZSH_FILES := $(wildcard shell/*.zsh)
GO_MODULES := $(patsubst %/go.mod,%,$(wildcard core/go.mod) $(wildcard tools/*/go.mod))
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help lint test verify build skill-check shell-check python-check go-check diff-check

help:
	@printf '%s\n' \
		'make build   Build bin/hq, the core binary' \
		'make lint    Validate shell, Python, and skill sources' \
		'make test    Run Go tests in every module' \
		'make verify  Run the complete local, non-destructive check'

build:
	@mkdir -p bin
	@cd core && go build \
		-ldflags '-X github.com/Gerc0g/dotfiles/core/internal/cli.Version=$(VERSION)' \
		-o ../bin/hq ./cmd/hq
	@echo 'built bin/hq ($(VERSION))'

lint: shell-check python-check skill-check

test: go-check

verify: lint test diff-check

shell-check:
	@set -e; for file in $(BASH_FILES); do \
		if head -n1 "$$file" | grep -q zsh; then zsh -n "$$file"; else bash -n "$$file"; fi; \
	done
	@set -e; for file in $(ZSH_FILES); do zsh -n "$$file"; done
	@echo 'shell syntax: ok'

python-check:
	@python3 -c 'import ast, pathlib; files = [*pathlib.Path("scripts").glob("*.py"), *pathlib.Path("skills-stash/wiki/scripts").glob("*.py")]; [ast.parse(path.read_text(), filename=str(path)) for path in files]; print(f"python syntax: ok ({len(files)} files)")'

skill-check: build
	@./bin/hq skill doctor

go-check:
	@set -e; for dir in $(GO_MODULES); do \
		echo "go test: $$dir"; \
		(cd "$$dir" && go test ./...); \
	done

diff-check:
	@git diff --check
	@echo 'git diff: ok'
