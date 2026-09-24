# kdbx-cli-tests

Integration tests for [kdbx-cli](https://github.com/dimkarp93/kdbx-cli), kept outside the tool so that it stays small and dependency-free. The tests treat `kdbx-cli` as a black box: they run the released binary against real tools and real services in Docker and check every secret delivery channel (`secrets`, `stdin`, `files`, `askpass`) together with the security invariants.

## Which version is tested

`versions.txt` holds the `kdbx-cli` version (`X.Y.Z`, the same format as in kdbx-cli). While the runner image is built, the binary is installed from the GitHub release `v<version>` with `github_install.sh` (the SHA256 is verified), and the tests refuse to start if `kdbx-cli --version` reports anything else. To test a new release, change `versions.txt`.

## Requirements

Docker with compose v2, `make`, `openssl`. Go is only needed on the host for `make vet` and `make vendor`.

## Running

```sh
make test                        # everything: services + both suites (~1 min)
make test MASK=TestAskSSH        # only tests whose name matches the regexp
make test-basic                  # the quick suite without services (only the runner container)
make test TRANSCRIPT=1           # record what every test did (see below)
make test VERBOSE=1              # go test -v
make up                          # bring the services up and seed them (for debugging)
make shell                       # a shell in the runner container, with a real TTY
make logs                        # service logs
make down                        # stop and remove everything
```

`make test` exits with the tests' status. On failure the service logs are saved into `_logs/compose.log`. A failed test prints the command's exit code, stdout and stderr.

## Recording: `TRANSCRIPT=1`

Everything goes into `_logs/transcripts/`:

- `SUMMARY.txt` — the tested `kdbx-cli` version, then every test with PASS/FAIL, its duration and its log file;
- `<Test>.log` — a readable protocol: each command with its arguments, the environment it added, stdin, stdout, stderr, exit code and timing, plus the test's own notes (requests seen by the GitHub mock, what the security scans covered) and the result;
- `<Test>.cast` — a terminal recording of PTY scenarios (the password prompt, `psql`, `ssh`) in the asciinema format: `asciinema play -s 0.2 _logs/transcripts/TestTTY1_MasterPasswordFromTerminal.cast`, or convert to a GIF with `agg`. Keystrokes are not shown on playback (the password is typed with echo off); they are listed in the `.log`.

Secrets are masked everywhere: a store entry shows up as `<secret:Title>`, the master password as `<master-password>`, service passwords by the name of their variable. So `<secret:REG_TOKEN>|<secret:REG_TOKEN>` in stdout means the child really received that secret twice. The files can be shared; CI attaches them to every run as the `e2e-logs` artifact.

## What is covered

| Suite | Scenarios |
|---|---|
| `tests/basic` | env injection, section merging, flags, dry-run, `check`/`config`, every channel on `sh -c`, exit codes, wrong password |
| env | `psql` + `PGPASSWORD`, `github_install.sh` against a GitHub API mock, `gitea_install.sh` against Gitea, section selection, dry-run without network |
| stdin | `psql -W` without a TTY, `--stdin-keep-open`, `skopeo login --password-stdin`, `keepassxc-cli db-create`, `sudo -S` |
| files | `restic --password-file`, `gpg --passphrase-file`, `mariadb --defaults-extra-file`, `PGPASSFILE`, the ssh key via `ssh-add`, `ssh -i` (documents that ssh closes the descriptor) |
| askpass | `ssh` password and key passphrase, `git clone` over HTTP, `sudo -A`, `RESTIC_PASSWORD_COMMAND` |
| PTY | the master password prompt (input, wrong password, Ctrl+D/Ctrl+C), `psql` preferring the terminal, `SSH_ASKPASS_REQUIRE=force` |
| security | the secret is absent from every process's argv and from disk (for all channels), the askpass directory is `0700` and removed, the secret is in the child's environ but not in kdbx-cli's |
| negative | multi-line secret over stdin, missing placeholder, missing entry, the tool's exit code propagated, wrong GitHub token |

## Known failures on 0.9.1

`TestFileMySQL1_MultilineOptionFile`, `TestFilePG1_PGPassFile` and `TestFileSSH2_PrivateKeyThroughSSHAdd` fail on 0.9.1: the in-memory file is created with mode `0777`, which `mariadb`, libpq and `ssh-add` reject. The fix (mode `0600`) is in kdbx-cli but has not been released yet; these tests turn green once `versions.txt` points to a release that includes it.

## Layout

- `compose.yaml` — PostgreSQL, MariaDB, sshd, Gitea, registry, the GitHub API mock and the `runner` container;
- `runner/` — the runner image: client tools, `keepassxc-cli`, the `dimkarp93/install` installers, `kdbx-cli` from the release;
- `services/` — the GitHub mock (`mockgithub`, Go stdlib only), sshd, PostgreSQL init;
- `seed/` — Gitea seeding (user, token, private repository, release);
- `internal/harness/` — sandboxes, the config schema, `keepassxc-cli` helpers, transcripts;
- `tests/basic/`, `tests/docker/` — the tests (build tag `e2e`).

On GitHub the suite runs in `.github/workflows/e2e.yml` (on push, daily and manually).
