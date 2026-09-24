#!/bin/sh
set -eu
dir=$(cd "$(dirname "$0")/.." && pwd)
compose="docker compose -f $dir/compose.yaml"
E2E_GITEA_PW=$(sed -n 's/^E2E_GITEA_PW=//p' "$dir/.env")

if ! out=$($compose exec -T -u git gitea gitea admin user create --admin \
        --username alice --password "$E2E_GITEA_PW" --email alice@e2e.local \
        --must-change-password=false 2>&1); then
    case "$out" in
        *"already exists"*) ;;
        *) echo "$out" >&2; exit 1 ;;
    esac
fi

$compose run --rm -T runner sh /src/seed/runner-seed.sh
