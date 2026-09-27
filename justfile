version_file := "versions.txt"
mask := env_var_or_default("MASK", "")
demo := env_var_or_default("DEMO", "")
verbose := env_var_or_default("VERBOSE", "")
local := env_var_or_default("LOCAL", "")
demo_dir := "_logs/demo"
kdbx_cli_src := "../kdbx-cli"
kdbx_cli_local_src_dir := "runner/kdbx-cli-src"
kdbx_cli_repo := "https://github.com/dimkarp93/kdbx-cli.git"
secrets := "E2E_PG_ADMIN_PW E2E_PG_STDIN_PW E2E_SSH_PW E2E_REG_PW E2E_SUDO_PW"

kdbx_cli_version := if local != "" { `test -f ../kdbx-cli/versions.txt && tr -d '[:space:]' < ../kdbx-cli/versions.txt || echo NOTFOUND` } else { `tr -d '[:space:]' < versions.txt` }

export KDBX_CLI_VERSION := kdbx_cli_version
export KDBX_CLI_LOCAL := if local != "" { "1" } else { "0" }
export GOWORK := "off"
export GOFLAGS := "-mod=vendor"

compose := "docker compose -f compose.yaml"
go_test := "go test -count=1 -tags=e2e" + (if verbose != "" { " -v" } else { "" }) + (if mask != "" { " -run '" + mask + "'" } else { "" })
demo_mount := '-v "' + justfile_directory() + "/" + demo_dir + ':/demo"'

default:
    @just --list

# run the full suite: services + both test suites
[group('test')]
test:
    #!/usr/bin/env bash
    set +e
    {{compose}} --profile runner down -v --remove-orphans >/dev/null 2>&1
    rm -f .env
    just up && just prepare-demo
    status=$?
    if [ $status -eq 0 ]; then
        {{compose}} run --rm -T runner {{go_test}} ./...
        status=$?
    fi
    if [ $status -eq 0 ] && [ -n "{{demo}}" ]; then
        for tape in demo/*.tape; do
            [ -e "$tape" ] || continue
            echo "recording $tape"
            {{compose}} run --rm -T {{demo_mount}} runner vhs "$tape" || status=$?
        done
    fi
    if [ $status -ne 0 ]; then
        mkdir -p _logs
        {{compose}} logs --no-color > _logs/compose.log 2>&1
        echo "compose logs: _logs/compose.log"
    fi
    if [ -n "{{demo}}" ]; then echo "demo: {{demo_dir}}/"; fi
    {{compose}} --profile runner down -v --remove-orphans >/dev/null 2>&1
    rm -f .env
    exit $status

# run the quick suite without services (runner container only)
[group('test')]
test-basic: env stage-local-src
    #!/usr/bin/env bash
    set +e
    {{compose}} build runner
    {{compose}} run --rm -T --no-deps runner {{go_test}} ./tests/basic/...
    exit $?

# run the docker-dependent suite only: services + tests/docker (no tests/basic, no demo)
[group('test')]
test-docker:
    #!/usr/bin/env bash
    set +e
    {{compose}} --profile runner down -v --remove-orphans >/dev/null 2>&1
    rm -f .env
    just up
    status=$?
    if [ $status -eq 0 ]; then
        {{compose}} run --rm -T runner {{go_test}} ./tests/docker/...
        status=$?
    fi
    if [ $status -ne 0 ]; then
        mkdir -p _logs
        {{compose}} logs --no-color > _logs/compose.log 2>&1
        echo "compose logs: _logs/compose.log"
    fi
    {{compose}} --profile runner down -v --remove-orphans >/dev/null 2>&1
    rm -f .env
    exit $status

# run the full suite with a demo recorded to _logs/demo/
[group('test')]
test-demo:
    DEMO=1 just test

# same as test, but builds kdbx-cli from ../kdbx-cli instead of installing the versions.txt release
[group('test')]
test-local:
    LOCAL=1 just test

# same as test-basic, but builds kdbx-cli from ../kdbx-cli instead of installing the versions.txt release
[group('test')]
test-basic-local:
    LOCAL=1 just test-basic

# same as test-docker, but builds kdbx-cli from ../kdbx-cli instead of installing the versions.txt release
[group('test')]
test-docker-local:
    LOCAL=1 just test-docker

# same as test-demo, but builds kdbx-cli from ../kdbx-cli instead of installing the versions.txt release
[group('test')]
test-demo-local:
    LOCAL=1 DEMO=1 just test

# list which scenarios each of test-basic/test-docker/test-demo covers, side by side
[group('report')]
list-tests:
    #!/usr/bin/env bash
    set -euo pipefail
    mapfile -t basic < <(grep -h '^func Test' tests/basic/*.go | grep -v '^func TestMain' | sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/')
    mapfile -t docker < <(grep -h '^func Test' tests/docker/*.go | grep -v '^func TestMain' | sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/')
    shopt -s nullglob
    demo=(demo/*.tape)
    demo=("${demo[@]##*/}")
    demo=("${demo[@]%.tape}")
    n=${#basic[@]}
    [ ${#docker[@]} -gt "$n" ] && n=${#docker[@]}
    [ ${#demo[@]} -gt "$n" ] && n=${#demo[@]}
    w1=5; w2=6; w3=4
    for s in "${basic[@]}"; do [ ${#s} -gt "$w1" ] && w1=${#s}; done
    for s in "${docker[@]}"; do [ ${#s} -gt "$w2" ] && w2=${#s}; done
    for s in "${demo[@]}"; do [ ${#s} -gt "$w3" ] && w3=${#s}; done
    printf "%-${w1}s  %-${w2}s  %-${w3}s\n" "BASIC" "DOCKER" "DEMO"
    for ((i = 0; i < n; i++)); do
        printf "%-${w1}s  %-${w2}s  %-${w3}s\n" "${basic[i]:-}" "${docker[i]:-}" "${demo[i]:-}"
    done

# list the demo aliases openable via `just demo <alias>`, with what each one checks
[group('report')]
list-demos:
    #!/usr/bin/env bash
    set -euo pipefail
    shopt -s nullglob
    tapes=(demo/*.tape)
    if [ ${#tapes[@]} -eq 0 ]; then
        echo "no demo/*.tape scenarios found" >&2
        exit 1
    fi
    for f in "${tapes[@]}"; do
        name=$(basename "$f" .tape)
        status="run: just test-demo"
        [ -f "{{demo_dir}}/$name.gif" ] && status="rendered"
        desc="$(grep -m1 '^# Demo: ' "$f" | sed 's/^# Demo: //')"
        printf '%-18s (%-16s) %s\n' "$name" "$status" "${desc:-no description}"
    done

# open a recorded VHS demo (see demo/*.tape); name is the tape's basename, e.g. master-password
[group('report')]
demo name:
    #!/usr/bin/env bash
    set -euo pipefail
    f="{{demo_dir}}/{{name}}.gif"
    if [ ! -f "$f" ]; then
        echo "no demo named '{{name}}'; see: just list-demos" >&2
        exit 1
    fi
    if command -v open >/dev/null; then
        open "$f"
    elif command -v xdg-open >/dev/null; then
        xdg-open "$f" >/dev/null 2>&1 &
    else
        echo "no GUI opener found (open/xdg-open); file: $f"
    fi

# extract the last frame of every recorded demo/*.gif as a .png screenshot
[group('report')]
make-screens: env
    #!/usr/bin/env bash
    set -euo pipefail
    shopt -s nullglob
    gifs=({{demo_dir}}/*.gif)
    if [ ${#gifs[@]} -eq 0 ]; then
        echo "no demo recorded yet; run: just test-demo" >&2
        exit 1
    fi
    {{compose}} run --rm -T --no-deps {{demo_mount}} runner bash -c '
        set -euo pipefail
        for f in /demo/*.gif; do
            name=$(basename "$f" .gif)
            ffmpeg -y -v error -i "$f" -vf reverse -frames:v 1 "/demo/$name.png"
        done
    '
    ls {{demo_dir}}/*.png | xargs -n1 basename

# open a demo's screenshot (see make-screens); name is the same as for `just demo`
[group('report')]
screen name:
    #!/usr/bin/env bash
    set -euo pipefail
    f="{{demo_dir}}/{{name}}.png"
    if [ ! -f "$f" ]; then
        echo "no screenshot for '{{name}}'; run: just make-screens" >&2
        exit 1
    fi
    if command -v open >/dev/null; then
        open "$f"
    elif command -v xdg-open >/dev/null; then
        xdg-open "$f" >/dev/null 2>&1 &
    else
        echo "no GUI opener found (open/xdg-open); file: $f"
    fi

# generate .env with random service secrets (skipped if it already exists)
[group('env')]
env:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -f .env ]; then exit 0; fi
    : > .env
    chmod 600 .env
    for k in {{secrets}}; do
        printf '%s=%s\n' "$k" "$(openssl rand -hex 16)" >> .env
    done

# recreate _logs/demo/ writable by the runner (only when DEMO=1)
[group('env')]
prepare-demo:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{demo}}" ]; then
        rm -rf {{demo_dir}}
        mkdir -p {{demo_dir}}
        chmod 0777 {{demo_dir}}
    fi

# stage ../kdbx-cli into runner/kdbx-cli-src for a local build (only when LOCAL=1); errors if not found
[group('env')]
stage-local-src:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf {{kdbx_cli_local_src_dir}}
    mkdir -p {{kdbx_cli_local_src_dir}}
    if [ -n "{{local}}" ]; then
        if [ "{{kdbx_cli_version}}" = "NOTFOUND" ] || [ ! -d "{{kdbx_cli_src}}" ]; then
            echo "LOCAL=1: no kdbx-cli source found at {{kdbx_cli_src}} (expected a sibling checkout next to this repo)" >&2
            exit 1
        fi
        rsync -a --exclude=.git --exclude=/kdbx-cli {{kdbx_cli_src}}/ {{kdbx_cli_local_src_dir}}/
        echo "building kdbx-cli locally from {{kdbx_cli_src}} (version {{kdbx_cli_version}})"
    else
        touch {{kdbx_cli_local_src_dir}}/.gitkeep
    fi

# bring the services up (for debugging)
[group('env')]
up: env stage-local-src
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{local}}" ]; then
        echo "kdbx-cli $KDBX_CLI_VERSION (local build from {{kdbx_cli_src}})"
    else
        echo "kdbx-cli $KDBX_CLI_VERSION (versions.txt)"
    fi
    {{compose}} up -d --build --wait
    {{compose}} build runner

# open a shell in the runner container, with a real TTY
[group('env')]
shell:
    {{compose}} run --rm runner bash

# show service logs
[group('env')]
logs:
    {{compose}} logs

# stop and remove services, volumes and .env
[group('env')]
down:
    {{compose}} --profile runner down -v --remove-orphans
    rm -f .env

# run go vet on the base and e2e-tagged code
[group('dev')]
vet:
    go vet ./...
    go vet -tags=e2e ./...

# update go.mod/go.sum and re-vendor dependencies
[group('dev')]
vendor:
    GOFLAGS= go mod tidy
    GOFLAGS= go mod vendor

# fail if vendor/ is out of sync with go.mod
[group('dev')]
vendor-check:
    GOFLAGS= go mod vendor
    test -z "$(git status --porcelain -- go.mod go.sum vendor/ | tee /dev/stderr)"

[group('dev')]
bump-version:
    #!/usr/bin/env bash
    set -euo pipefail
    latest_tag="$(git ls-remote --tags --refs {{kdbx_cli_repo}} \
        | awk -F/ '{print $NF}' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
        | sort -V \
        | tail -n1)"
    if [ -z "$latest_tag" ]; then
        echo "no version tags found in {{kdbx_cli_repo}}" >&2
        exit 1
    fi
    latest_version="${latest_tag#v}"
    current_version="$(tr -d '[:space:]' < {{version_file}})"
    if [ "$latest_version" = "$current_version" ]; then
        echo "{{version_file}} already at $current_version"
        exit 0
    fi
    printf '%s\n' "$latest_version" > {{version_file}}
    echo "{{version_file}}: $current_version -> $latest_version"
