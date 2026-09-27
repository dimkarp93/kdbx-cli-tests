# CLAUDE.md

## Code style

**Do not write comments in the code.** The code must be self-documenting through identifier names. The only exception is a short "why" line where the reason would otherwise be invisible (a hidden invariant, a workaround).

## Language

All code, scripts, test messages and CI configs are **English only**. Documentation is kept as file pairs: `<name>.ru.md` (the source of truth), `<name>.en.md`, and `<name>.md` — a symlink to `<name>.en.md`. Changing a document means editing both versions.

## Purpose

Black-box integration tests for `kdbx-cli`. The tested version is `versions.txt` (`X.Y.Z`); the runner image installs that GitHub release (`dimkarp93/kdbx-cli`, tag `v<version>`) with `github_install.sh`, and `harness.CheckBinary` refuses to run if `kdbx-cli --version` differs. The tests never import kdbx-cli code: the config schema is duplicated in `internal/harness/kdbx.go` on purpose, as a contract.

## Structure

- `compose.yaml` — the `postgres` and `sshd` services (only what demos and `tests/docker` need) and `runner` (profile `runner`, run with `-T`, no TTY). `KDBX_CLI_VERSION` comes from the justfile.
- `runner/` — `golang:1.26-trixie` + client tools + `keepassxc-minimal` + `vhs`/`ttyd`/`ffmpeg`/`chromium` for demo recordings (`VHS_NO_SANDBOX=1`); the entrypoint sets the `tester` sudo password and drops privileges.
- `demo/` — `setup.sh` builds a demo store from the `.env` secrets, `*.tape` — `vhs` scenarios (a real interactive terminal session, not an excerpt of a go-test protocol); rendered to `_logs/demo/*.gif` by `just test-demo`.
- `services/` — PostgreSQL init (`postgres/init.sh`, the `app_stdin` role), sshd.
- `internal/harness` — `Sandbox` (per-test `HOME`, `MakeStore`, `WriteConfig`, `StoreTitles`, `Run*`/`Exec` with a 30 s timeout), `Demo`/`Cast` (per-test protocols and asciinema recordings, secrets masked), `CheckBinary`, `WriteSummaryHeader`, the config schema and `keepassxc-cli` helpers.
- `tests/basic` — the quick suite (no services, most scenarios), `tests/docker` — only what can't be checked without postgres/sshd (`TestMain` waits for those two). The two never overlap. Build tag `e2e`.

## Commands

`[MASK=…] [LOCAL=1] [VERBOSE=1] just test` (basic+docker, no demo, guarantees it all works), `just test-basic`, `just test-docker` (only the service-dependent scenarios), `just test-demo` (separate: run the suite, then record the demo), `*-local` variants of all four (kdbx-cli from `../kdbx-cli`, not `versions.txt`), `just list-tests` (what's in basic/docker/demo, in 3 columns), `just list-demos`, `just demo <name>`, `just make-screens`, `just screen <name>`, `just up/shell/logs/down`, `just vet`, `just vendor`/`vendor-check` (`GOFLAGS=-mod=vendor`). Service secrets are generated into `.env` for each run; `_logs/` holds compose logs, protocols and demo recordings (`.gif`).
