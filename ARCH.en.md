# kdbx-cli-tests architecture

This document explains how the tests are built, what they depend on and how the recording (demo) works. For how to run them, see `README.en.md`.

## 1. Principles

- **Black box.** The tests never import `kdbx-cli` code. They run the released binary and look only at observable behaviour: stdout, stderr, exit code, what the child program received, what the service saw.
- **The release is tested, not the sources.** The version comes from `versions.txt`; the binary is taken from the GitHub release tagged `v<version>`.
- **Real tools and services.** The delivery channels are exercised with real `psql`, `ssh`, `git`, `restic`, `gpg`, `sudo`, `keepassxc-cli`. The only network services are `postgres` and `sshd`, and only where a real one is needed to prove the invariant (`tests/docker`, §5); anything provable locally lives in `tests/basic` with no docker service at all.
- **Determinism and isolation.** Every run starts from a clean stack and fresh random passwords. Every test has its own `HOME`, every command has a timeout. No outbound network is needed while the tests run: everything is downloaded when the images are built.
- **No silent skips.** If a required tool is missing inside the runner, the test fails instead of being skipped. This keeps the image from degrading silently.

## 2. Overview

```
 host                               docker compose (network e2e, project kdbx-cli-tests)
 ────                               ──────────────────────────────────────────────────
 just test ──► .env (secrets) ──┬─► postgres   sshd
     │                          │      ▲         ▲
     │                          │      └─────────┘ real protocols
     │                          └─► runner (no TTY, -T)
     │                               go test ./... ──► kdbx-cli (release) ──► psql/ssh/git/...
     │                                    │
     └──────── _logs/ ◄───────────────────┘  compose.log, demo/ (bind mount)
```

The runner is the only container that executes the Go test code. The services are brought up first with `up --wait`; the runner is started separately with `compose run --rm -T runner …`.

## 3. Four commands, three non-overlapping groups

- **`just test-basic`** — only `tests/basic`, `compose run --no-deps`: no services are brought up at all.
- **`just test-docker`** — only `tests/docker`, with `postgres`+`sshd`.
- **`just test`** — the union of both groups, one `go test ./...` inside the stack: guarantees everything works, without recording a demo.
- **`just test-demo`** — stands apart: `DEMO=1 just test`, i.e. the whole `test` first, and only if it passed — rendering `demo/*.tape` (see §8).

The lifecycle of `just test` (shared by `test`/`test-demo`; `test-docker` is the same minus steps 4/6; `test-basic` skips services entirely):

1. `down -v` and delete `.env`: a clean start even if the previous run was interrupted.
2. `just env` generates `.env` (`0600`): `openssl rand -hex 16` for every variable in `SECRETS`.
3. `just up`: `compose up -d --build --wait` brings up `postgres`+`sshd` and waits for their healthcheck, then `compose build runner`.
4. `prepare-demo` (only with `DEMO=1`) recreates `_logs/demo/` with mode `0777`, so the `tester` user inside the container can write to it.
5. `compose run --rm -T runner go test -count=1 -tags=e2e ./...` (or `./tests/docker/...` for `test-docker`, `./tests/basic/...` for `test-basic`), with `MASK` and `VERBOSE` applied.
6. If the tests passed and `DEMO=1`: `compose run --rm -T runner vhs demo/<name>.tape` for each `demo/*.tape`, one at a time — the services are still up, so scenarios like `askpass-ssh`/`psql-terminal` connect to the real `sshd`/`postgres` (see §8, "VHS recordings").
7. If something failed (the tests or a demo recording), `compose logs` is saved to `_logs/compose.log`.
8. `down -v` and delete `.env`: containers, volumes and passwords are gone. The exit code of `just` is the exit code of the tests.

The `*-local` variants of all four (`test-local`, `test-basic-local`, `test-docker-local`, `test-demo-local`) are identical, but build `kdbx-cli` from `../kdbx-cli` instead of the release (see §4).

## 4. The kdbx-cli version

```
versions.txt (0.9.1)
  └► justfile: export KDBX_CLI_VERSION := `tr -d '[:space:]' < versions.txt`
       └► compose.yaml: runner.build.args.KDBX_CLI_VERSION and runner.environment.KDBX_CLI_VERSION (${…:?})
            └► runner/Dockerfile: github_install.sh -s https://github.com dimkarp93/kdbx-cli kdbx-cli "$KDBX_CLI_VERSION"
                 (downloads kdbx-cli-linux-<arch>.tar.gz and SHA256SUMS, verifies the checksum, installs into /usr/local/bin,
                  then test "$(kdbx-cli --version)" = "$KDBX_CLI_VERSION")
                      └► TestMain → harness.CheckBinary(): checks --version against KDBX_CLI_VERSION again
```

The version is checked twice, at image build time and at test start. This protects against a cached image with an old binary.

### `LOCAL=1`: building from source

```
LOCAL=1 just test (or just test-local / test-basic-local / test-demo-local)
  └► just: kdbx_cli_version reads ../kdbx-cli/versions.txt instead of this repo's versions.txt
       (no ../kdbx-cli, or it has no versions.txt → kdbx_cli_version = NOTFOUND)
  └► stage-local-src: NOTFOUND or no ../kdbx-cli → a clear error and exit 1
       otherwise → rsync ../kdbx-cli (without .git and the kdbx-cli binary) into runner/kdbx-cli-src/ (gitignored)
  └► compose.yaml: runner.build.args.KDBX_CLI_LOCAL=1
       └► runner/Dockerfile: COPY runner/kdbx-cli-src /opt/kdbx-cli-src
            RUN cd /opt/kdbx-cli-src && GOFLAGS=-mod=vendor go build -o /usr/local/bin/kdbx-cli ./cmd/kdbx-cli
            (the same -ldflags scheme as kdbx-cli's own Makefile, channel=local)
```

Without `LOCAL=1`, `stage-local-src` just clears `runner/kdbx-cli-src/` down to an empty `.gitkeep` — the Dockerfile takes the `github_install.sh` branch as before, and the `COPY` layer isn't invalidated (the directory's content is byte-identical across ordinary runs).

## 5. Services

Exactly two — only what can't be proven without a real network (§9 explains why the rest is deliberately not set up):

| Service | Image | What is exercised | Readiness | Used by |
|---|---|---|---|---|
| `postgres` | `postgres:17.6-alpine` | the `app_stdin` role's password (SCRAM); the role is created by `services/postgres/init.sh` from `E2E_PG_STDIN_PW` | `pg_isready -h 127.0.0.1` | STDIN-PG-3 (the `psql-terminal` demo) |
| `sshd` | custom, `alpine:3.22` + openssh | the `alice` password, keys from `/seed/ssh/authorized_keys` (`StrictModes no`) | `nc -z 127.0.0.1 22` | ASK-SSH-3 (the `askpass-ssh` demo), FILE-SSH-1/2 |
| `runner` | custom, `golang:1.26-trixie` | where the tests run | — (profile `runner`) | all tests |

The `psql-terminal` demo creates its own `app_demo` role (a fixed password, created by `demo/setup.sh` via the postgres admin) — kept separate from `app_stdin` so it never touches the real test's randomized password.

### The runner (`runner/`)

- Base image `golang:1.26-trixie`: Go is needed to run `go test` inside the container.
- apt clients: `keepassxc-minimal` (provides `keepassxc-cli`), `postgresql-client`, `openssh-client`, `git`, `restic`, `gnupg`, `sudo`, `curl`, `jq`, `procps`, `util-linux`, `netcat-openbsd`, `openssl`, plus `ffmpeg`/`chromium` and a static `ttyd` for demo recordings (§8).
- The `github_install.sh` installer is downloaded from commit `INSTALL_REF` into `/opt/install/` — needed both to install `kdbx-cli` itself from the release and for the `EnvGH3` test (dry-run, no network).
- The `tester` user has password-protected sudo with `Defaults timestamp_timeout=0`, so the sudo cache cannot hide problems.
- The test sources are copied into `/src`. `GOFLAGS=-mod=vendor`, `GOCACHE=/tmp/gocache`, `KDBX_CLI_BIN=/usr/local/bin/kdbx-cli`, `KDBX_CLI_E2E_ROOT=/work`, `E2E_IN_DOCKER=1`.
- `entrypoint.sh` starts as root: it sets the `tester` password from `E2E_SUDO_PW`, gives `/seed` to `tester` and drops privileges with `setpriv`.

The shared `seed` volume is mounted on `runner` and `sshd` — the only thing in it now is `/seed/ssh/authorized_keys` (created by the `sshd` container itself at startup), which the FILE-SSH/ASK-SSH tests append their keys to.

## 6. Code layout

```
internal/harness/            shared code, no build tag
  sandbox.go                 Sandbox, NewSandbox, SafeName, MakeStore, WriteConfig, ConfigPath, StoreTitles,
                             BaseEnv, Exec, RunEnv, Run, RunStdin, RunWithPassword, RunNoPassword, Result
  kdbx.go                    Config/Section (the kdbx-cli config JSON schema), LoadConfig, KeepassRun, ExportTitles
  binary.go                  CheckBinary, KDBX_CLI_BIN / KDBX_CLI_VERSION
tests/basic/                 the quick suite (tag e2e), no services — most scenarios
  basic_test.go              TestMain (no service wait), newSandbox, E2E-*
  helpers_test.go            secret, requireTool, expectOK/expectFail, storeWith, resticRepo, runtimeSandbox
  pty_test.go                ptySession (creack/pty), terminal query answers, TTY-1..3
  env_test.go, multi_env_test.go, file_test.go, ssh_test.go, stdin_test.go, negative_test.go, security_test.go
tests/docker/                only what can't be checked without postgres/sshd (tag e2e)
  harness_test.go            TestMain (waits for postgres+sshd), newSandbox, secret, requireTool, expectOK/expectFail
  pty_test.go                startPTY, STDIN-PG-3, ASK-SSH-3
  ssh_test.go                sshPasswordArgs/sshKeyArgs/authorizedKey, FILE-SSH-1/2
services/                    postgres/init.sh (the app_stdin role), sshd/ (Dockerfile, entrypoint.sh, sshd_config)
demo/                        setup.sh (demo store + the app_demo postgres role) + *.tape (vhs scenarios)
```

### The test sandbox

`harness.NewSandbox(t, bin)` creates `/work/<SafeName(t.Name())>/` (on the host, under `KDBX_CLI_E2E_ROOT`):

- `home/` — `HOME` for every command of the test, containing `home/.config/kdbx-cli/default` (the config) and `home/store.kdbx`;
- `run/` (`0700`) — `XDG_RUNTIME_DIR`, where kdbx-cli puts its askpass directory;
- the directory is removed in `t.Cleanup` unless `KDBX_CLI_E2E_KEEP=1`.

`MakeStore` builds a KeePass XML file, imports it with `keepassxc-cli import` using the master password `TestPassword`, and deletes the XML right away so no plaintext stays on disk (this matters for SEC-DISK). kdbx-cli gets the master password through `KDBX_CLI_PASSWORD`, except in PTY tests, where it is typed at the terminal.

`Exec` is the single entry point for running commands: `exec.CommandContext` with a 30 s timeout and a 2 s `WaitDelay`. stdout and stderr are captured; on timeout the test fails with the partial output. Every `Run*` and `sh` is a wrapper around it.

### PTY

`ptySession` runs a process under a 200×40 pseudo-terminal (`pty.StartWithSize`), reads its output in the background, and provides `expect` (substring with a timeout), `send`, `wait`, `mustExit` and `kill`. Under a terminal kdbx-cli (termenv/lipgloss) asks for the background colour (`OSC 11`) and the cursor position (`CSI 6n`). The helper answers these queries the way a real terminal does; otherwise every run would spend 5 s waiting.

## 7. Dependencies

### Go

| Module | Where | Why |
|---|---|---|
| stdlib | everywhere | the whole harness, the tests |
| `github.com/creack/pty` | `tests/basic/pty_test.go`, `tests/docker/pty_test.go` | pseudo-terminal for TTY scenarios |
| `golang.org/x/sys/unix` | `tests/basic/pty_test.go` | reading termios (echo after Ctrl+C) |

The dependencies are vendored (`GOFLAGS=-mod=vendor`), so `go test` in the runner does not touch the network. `just vendor-check` makes sure `vendor/` matches `go.mod`.

Package graph: `tests/basic` → `internal/harness`, `creack/pty`, `x/sys`; `tests/docker` → `internal/harness`, `creack/pty`. No `kdbx-cli` code is imported. The config schema is duplicated in `kdbx.go` on purpose: if the format changes in the tool, the tests will notice.

### What is downloaded at build time

| What | From | Pinned by |
|---|---|---|
| `kdbx-cli` | GitHub release `dimkarp93/kdbx-cli` | `versions.txt` + SHA256SUMS |
| `github_install.sh` installer | raw.githubusercontent.com `dimkarp93/install` | `INSTALL_REF` (a commit) |
| client tools | Debian trixie apt | distribution release |
| `vhs` | `go install` | unpinned (`@latest`) |
| `ttyd` | GitHub release `tsl0922/ttyd` | `TTYD_VERSION` |
| service images | Docker Hub | exact version tags |

### Tests and services

| Group | postgres | sshd |
|---|---|---|
| `tests/basic` (everything else) | | |
| `tests/docker`: STDIN-PG-3 | ✓ | |
| `tests/docker`: ASK-SSH-3, FILE-SSH-1/2 | | ✓ |

`TestMain` in `tests/docker` waits for `postgres:5432` and `sshd:22` from `serviceAddrs`. `tests/basic` waits for nothing — it has no services.

### Secrets

```
justfile (openssl rand) ─► .env ─► runner env ─► secret(t, "E2E_…") ─► MakeStore(.kdbx) ─► kdbx-cli ─► tool ─► service
```

There are no hard-coded passwords. A test puts into the `.kdbx` the same value the service was configured with (or one it created itself, like `demo/setup.sh`'s `app_demo` role), so a successful login proves unambiguously that the secret was delivered. `E2E_REG_PW` (a historical name) is no longer tied to any service — a few `tests/basic` tests (restic, gpg, keepassxc) just use it as a source of a random-looking string.

### Shared state and ordering

The tests do not use `t.Parallel()` and run sequentially. The state they share is:

- `/seed/ssh/authorized_keys` (tests append their keys);
- files in `/etc` written by the sudo tests (unique names).

## 8. Demo recording (VHS)

Recording only happens with `DEMO=1` (`just test-demo`), after `go test` has already passed, and it never touches the Go test code: the tests only run and check behaviour, nothing they do is captured. Recording is entirely `demo/*.tape` scenarios run through [`vhs`](https://github.com/charmbracelet/vhs):

```
DEMO=1 just test (after go test has already passed)
  └► prepare-demo: _logs/demo/ (0777)
  └► for tape in demo/*.tape:
       compose run -v _logs/demo:/demo runner vhs "$tape"
         └► demo/setup.sh (in a Hide block) builds /tmp/demo-store.kdbx: ssh-pw from E2E_SSH_PW, pg-stdin — a fixed
            password for a new app_demo role (created there via the postgres admin, kept separate from the
            test's randomized app_stdin)
         └► a real interactive session in a headless terminal (ttyd) is captured frame by frame through
            a headless Chromium (xterm.js) and assembled into a GIF with ffmpeg
```

`runner/Dockerfile` installs `vhs` (`go install`), a static `ttyd` binary (no apt package on trixie), `ffmpeg`, `chromium`; `VHS_NO_SANDBOX=1` is required — Chromium refuses to start in the container without it. `Output` in `.tape` files must be quoted (`Output "/demo/name.gif"`): an unquoted absolute path fails to parse with this vhs version; the same goes for double quotes nested inside `Type "…"` — an escaped `\"…\"` fails to parse, so use a different quote character inside a string that needs one (e.g. a SQL literal).

Every `.tape` ends with `Sleep 15s` on the final frame — enough time for a viewer to read the result before the GIF loops.

`just list-demos` lists the aliases (a `.tape`'s basename) and whether a `.gif` has already been rendered for it; `just demo <name>` opens the resulting `_logs/demo/<name>.gif` with `open`/`xdg-open`.

There is no verification here: `vhs` renders whatever the scenario types, it does not check `kdbx-cli`'s output against anything. If real behaviour changes, the `.tape` still "succeeds" and produces a GIF — just possibly a wrong one. The only actual checks live in the Go tests (§6); demo scenarios duplicate the same steps by hand and are not kept in sync with the tests automatically.

## 9. Extending

- **A new scenario without services.** A file in `tests/basic/` with the `e2e` tag. `newSandbox` already lives in `basic_test.go`; `secret`/`requireTool`/`expectOK`/`expectFail`/`storeWith`/`resticRepo` live in `helpers_test.go`. This is the default: write here whenever the scenario can be proven at all without the network.
- **A new scenario on `postgres`/`sshd`.** A file in `tests/docker/` with the `e2e` tag, only when a real service is the only way to prove the invariant (see §5) — otherwise it duplicates an already-proven channel and belongs in `tests/basic`. `requireTool`, `newSandbox`, `sb.Run*`; for PTY, use `startPTY`.
- **A new service.** Before adding one, check whether the same thing can be proven locally instead (see the duplication note above). If it's genuinely needed: add it to `compose.yaml` with a healthcheck, to the runner's `depends_on` and to `serviceAddrs` (`tests/docker/harness_test.go`). If it needs a password, add a variable to `SECRETS` in the justfile.
- **A new kdbx-cli version.** Change `versions.txt`. If the behaviour changed, update the tests in the same commit.
- **A new VHS demo.** A `demo/<name>.tape` file; do setup in a `Hide` block (`bash demo/setup.sh`) so the viewer never sees it; `Output` must be quoted. `just test-demo` picks the file up automatically, `just demo <name>` opens the result, `just list-tests` shows it in the DEMO column.
