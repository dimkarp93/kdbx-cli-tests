# kdbx-cli-tests

Integration tests for [kdbx-cli](https://github.com/dimkarp93/kdbx-cli), kept outside the tool so that it stays small and dependency-free. The tests treat `kdbx-cli` as a black box: they run the released binary against real tools and real services in Docker and check every secret delivery channel (`secrets`, `stdin`, `files`, `askpass`) together with the security invariants.

## Which version is tested

`versions.txt` holds the `kdbx-cli` version (`X.Y.Z`, the same format as in kdbx-cli). While the runner image is built, the binary is installed from the GitHub release `v<version>` with `github_install.sh` (the SHA256 is verified), and the tests refuse to start if `kdbx-cli --version` reports anything else. To test a new release, change `versions.txt`.

`LOCAL=1` (or the `*-local` recipes) builds `kdbx-cli` from source in `../kdbx-cli` instead — a sibling checkout next to this repo (`~/tools/kdbx-cli` in the usual layout). The version then comes from `../kdbx-cli/versions.txt`, not this repo's `versions.txt`. If `../kdbx-cli` isn't found, the build fails immediately with a clear error.

## Requirements

Docker with compose v2, `just`, `openssl`. Go is only needed on the host for `just vet` and `just vendor`.

## Running

```sh
just test                        # everything without a demo: tests/basic + tests/docker (~1 min), guarantees it all works
MASK=TestAskSSH just test        # only tests whose name matches the regexp
just test-basic                  # the quick suite without services (only the runner container)
just test-docker                 # only the tests that need real services (postgres, sshd)
just test-demo                   # a separate run: the suite, then recording demo/*.tape (see below)
just test-local                  # same as just test, but builds kdbx-cli from ../kdbx-cli
just test-basic-local            # same for test-basic
just test-docker-local           # same for test-docker
just test-demo-local             # same for test-demo
VERBOSE=1 just test              # go test -v
just list-tests                  # which scenarios belong to test-basic/test-docker/test-demo, in 3 columns
just up                          # bring the services up (for debugging)
just shell                       # a shell in the runner container, with a real TTY
just logs                        # service logs
just down                        # stop and remove everything
```

`test-basic`, `test-docker` and `test` (their union) never overlap — each test belongs to exactly one group. `test` exits with the tests' status. On failure the service logs are saved into `_logs/compose.log`. A failed test prints the command's exit code, stdout and stderr.

## Demos: VHS recordings

`just test-demo` (`DEMO=1 just test`) first runs the whole suite (like `test`), then — only if the tests passed — runs the `demo/*.tape` scripts through [`vhs`](https://github.com/charmbracelet/vhs) and drops the result into `_logs/demo/*.gif`. This isn't an excerpt of a go-test protocol: it's an actual interactive terminal session (the command being typed, the real output of `kdbx-cli` and the program it drives, the password prompt) recorded to a GIF independently of the Go tests — its own setup (`demo/setup.sh`), its own passwords, no assertions and no link back to the tests. Each `.tape` first reveals the demo store's contents (`keepassxc-cli ls`/`show -a Password`, so you see both the entry's title and its value), then runs the scenario end to end, and holds the final frame for 15 seconds so there's time to read it.

```sh
just test-demo             # run the suite and record every demo/*.tape into _logs/demo/*.gif
just list-demos            # which aliases exist, what each checks, whether it's rendered
just demo master-password  # open the rendered GIF (xdg-open/open)
just make-screens          # extract a .png from the last frame of every demo/*.gif
just screen master-password # open that .png (xdg-open/open)
```

Currently defined:

| Name | What it shows |
|---|---|
| `master-password` | typing the correct master password at an interactive terminal prompt |
| `wrong-password` | typing a wrong master password and being rejected |
| `ctrl-c` | Ctrl+D is ignored at the password prompt, Ctrl+C aborts it |
| `askpass-ssh` | `--askpass` feeds the password into `ssh` without a terminal prompt |
| `psql-terminal` | `psql` ignores the `--stdin` secret, asks for a password at the terminal instead — and connects once it's typed there |
| `gpg-decrypt` | `--secret-file` writes the secret to a temp file and hands its path to `gpg --passphrase-file` |

To add a new one, drop a `demo/<name>.tape` next to them (see `vhs new` for the syntax) — `just test-demo` picks it up automatically.

## What is covered

`tests/docker` (the full suite with real services) is trimmed down to the scenarios that can't be checked without the network/docker **and** are either already shown in a demo (see above) or verify kdbx-cli's own invariant rather than "yet another tool" on an already-proven channel. Everything else lives in `tests/basic`, still covering most of the tools below, just without external services (the runner container alone).

| Suite | Scenarios |
|---|---|
| `tests/basic` | env/stdin/files/askpass on `sh -c`, dry-run, `check`/`config`, exit codes, wrong password, section selection by basename; PTY (the master password prompt: input, wrong password, Ctrl+D/Ctrl+C); `restic --password-file`/`--password-command`, `gpg --passphrase-file`, `sudo -A`/`-S`, `keepassxc-cli db-create`; negative cases (multi-line secret over stdin, missing placeholder); security (the secret is absent from argv/disk for every channel, the askpass directory is `0700` and removed, the secret is in the child's environ but not in kdbx-cli's) |
| `tests/docker` | ssh closes the secret file's descriptor (`ssh -i`) and accepts a key through `ssh-add`; `psql` ignores the `--stdin` secret and asks for a password at the terminal (the `psql-terminal` demo); `ssh` via `--askpass` under a real TTY (the `askpass-ssh` demo) — postgres and sshd only, no other services |

## Layout

- `compose.yaml` — PostgreSQL, sshd and the `runner` container;
- `runner/` — the runner image: client tools, `keepassxc-cli`, `vhs`/`ttyd`/`ffmpeg`/`chromium` for demos, `kdbx-cli` from the release (or built locally, see `LOCAL=1`);
- `services/` — PostgreSQL init, sshd;
- `internal/harness/` — sandboxes, the config schema, `keepassxc-cli` helpers;
- `demo/` — `vhs` scenarios (see "Demos: VHS recordings" above);
- `tests/basic/`, `tests/docker/` — the tests (build tag `e2e`).

On GitHub the suite runs in `.github/workflows/e2e.yml` (on push, daily and manually).
