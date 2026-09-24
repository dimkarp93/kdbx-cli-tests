VERSION_FILE := versions.txt
MASK :=
TRANSCRIPT :=
VERBOSE :=
TRANSCRIPTS := _logs/transcripts
SECRETS := E2E_PG_ADMIN_PW E2E_PG_ENV_PW E2E_PG_STDIN_PW E2E_PG_FILE_PW E2E_MYSQL_PW E2E_MYSQL_ROOT_PW E2E_SSH_PW E2E_REG_PW E2E_GH_TOKEN E2E_GITEA_PW E2E_SUDO_PW

export KDBX_CLI_VERSION := $(shell tr -d '[:space:]' < $(VERSION_FILE))
export GOWORK := off
export GOFLAGS := -mod=vendor

COMPOSE := docker compose -f compose.yaml
GO_TEST := go test -count=1 -tags=e2e $(if $(VERBOSE),-v) $(if $(MASK),-run '$(MASK)')
TRANSCRIPT_ARGS := $(if $(TRANSCRIPT),-v "$(CURDIR)/$(TRANSCRIPTS):/transcripts" -e KDBX_CLI_E2E_TRANSCRIPT_DIR=/transcripts)

.PHONY: help test test-basic up down shell logs env prepare-transcripts vet vendor vendor-check

help:
	@grep -E '^[a-zA-Z_-]+:' Makefile | sed 's/:.*//' | sort -u

env:
	@set -eu; \
	[ -f .env ] && exit 0; \
	: > .env; \
	chmod 600 .env; \
	for k in $(SECRETS); do \
		printf '%s=%s\n' "$$k" "$$(openssl rand -hex 16)" >> .env; \
	done

prepare-transcripts:
	@if [ -n "$(TRANSCRIPT)" ]; then \
		rm -rf $(TRANSCRIPTS); \
		mkdir -p $(TRANSCRIPTS); \
		chmod 0777 $(TRANSCRIPTS); \
	fi

up: env
	@echo "kdbx-cli $(KDBX_CLI_VERSION) (versions.txt)"
	$(COMPOSE) up -d --build --wait
	$(COMPOSE) build runner
	./seed/seed.sh

test:
	@set +e; \
	$(COMPOSE) --profile runner down -v --remove-orphans >/dev/null 2>&1; \
	rm -f .env; \
	$(MAKE) --no-print-directory up prepare-transcripts; \
	status=$$?; \
	if [ $$status -eq 0 ]; then \
		$(COMPOSE) run --rm -T $(TRANSCRIPT_ARGS) runner $(GO_TEST) ./...; \
		status=$$?; \
	fi; \
	if [ $$status -ne 0 ]; then \
		mkdir -p _logs; \
		$(COMPOSE) logs --no-color > _logs/compose.log 2>&1; \
		echo "compose logs: _logs/compose.log"; \
	fi; \
	if [ -n "$(TRANSCRIPT)" ]; then echo "transcripts: $(TRANSCRIPTS)/SUMMARY.txt"; fi; \
	$(COMPOSE) --profile runner down -v --remove-orphans >/dev/null 2>&1; \
	rm -f .env; \
	exit $$status

test-basic: env prepare-transcripts
	@set +e; \
	$(COMPOSE) build runner; \
	$(COMPOSE) run --rm -T --no-deps $(TRANSCRIPT_ARGS) runner $(GO_TEST) ./tests/basic/...; \
	status=$$?; \
	if [ -n "$(TRANSCRIPT)" ]; then echo "transcripts: $(TRANSCRIPTS)/SUMMARY.txt"; fi; \
	exit $$status

shell:
	$(COMPOSE) run --rm runner bash

logs:
	$(COMPOSE) logs

down:
	$(COMPOSE) --profile runner down -v --remove-orphans
	rm -f .env

vet:
	go vet ./...
	go vet -tags=e2e ./...

vendor:
	GOFLAGS= go mod tidy
	GOFLAGS= go mod vendor

vendor-check:
	GOFLAGS= go mod vendor
	test -z "$$(git status --porcelain -- go.mod go.sum vendor/ | tee /dev/stderr)"
