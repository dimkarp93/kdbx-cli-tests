# CLAUDE.md

## Code style

**Do not write comments in the code.** The code must be self-documenting through identifier names. The only exception is a short "why" line where the reason would otherwise be invisible (a hidden invariant, a workaround).

## Language

All code, scripts, test messages and CI configs are **English only**. Documentation is kept as file pairs: `<name>.ru.md` (the source of truth), `<name>.en.md`, and `<name>.md` — a symlink to `<name>.en.md`. Changing a document means editing both versions.

## Purpose

Black-box integration tests for `kdbx-cli`. The tested version is `versions.txt` (`X.Y.Z`); the runner image installs that GitHub release (`dimkarp93/kdbx-cli`, tag `v<version>`) with `github_install.sh`, and `harness.CheckBinary` refuses to run if `kdbx-cli --version` differs. The tests never import kdbx-cli code: the config schema is duplicated in `internal/harness/kdbx.go` on purpose, as a contract.

## Structure

- `compose.yaml` — services (postgres, mariadb, sshd, gitea, registry + registry-auth, mockgithub) and the `runner` (profile `runner`, run with `-T`, no TTY). `KDBX_CLI_VERSION` comes from the Makefile.
- `runner/` — `golang:1.26-trixie` + client tools + `keepassxc-minimal` + installers pinned by `INSTALL_REF`; the entrypoint sets the `tester` sudo password and drops privileges.
- `services/mockgithub` — a GitHub releases API mock (pretty JSON, asset `url` before `name`, 404 without the token, `/__requests`, `/__reset`, `/__echo`); `main_test.go` checks it against the installer's parsing.
- `seed/` — `seed.sh` (host: Gitea admin user) → `runner-seed.sh` (token, private repo, release, `/seed/gitea.env`, `/seed/ready`).
- `internal/harness` — `Sandbox` (per-test `HOME`, `MakeStore`, `WriteConfig`, `StoreTitles`, `Run*`/`Exec` with a 30 s timeout), `Transcript`/`Cast` (per-test protocols and asciinema recordings, secrets masked), `CheckBinary`, `WriteSummaryHeader`, the config schema and `keepassxc-cli` helpers.
- `tests/basic` — the quick suite (no services), `tests/docker` — the full suite (`TestMain` waits for services and `/seed/ready`). Build tag `e2e`.

## Commands

`make test [MASK=…] [TRANSCRIPT=1] [VERBOSE=1]`, `make test-basic`, `make up/shell/logs/down`, `make vet`, `make vendor`/`vendor-check` (`GOFLAGS=-mod=vendor`). Service secrets are generated into `.env` for each run; `_logs/` holds compose logs and transcripts.
