#!/bin/sh
set -eu

gitea=http://gitea:3000
repo=alice/private-tool
auth="alice:${E2E_GITEA_PW:?}"

case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

api() {
    method=$1
    path=$2
    shift 2
    curl -fsS -X "$method" -u "$auth" -H 'Content-Type: application/json' "$gitea/api/v1$path" "$@"
}

if [ ! -s /seed/gitea.env ]; then
    token=$(api POST /users/alice/tokens \
        -d '{"name":"e2e","scopes":["read:repository","write:repository"]}' | jq -r .sha1)

    api POST /user/repos \
        -d '{"name":"private-tool","private":true,"auto_init":true,"default_branch":"main"}' >/dev/null

    release_id=$(api POST "/repos/$repo/releases" \
        -d '{"tag_name":"v1.0.0","target_commitish":"main","name":"v1.0.0"}' | jq -r .id)

    work=$(mktemp -d)
    printf '#!/bin/sh\ncase "${1:-}" in\n--version) echo 1.0.0 ;;\n*) echo hello-ok ;;\nesac\n' > "$work/hello"
    chmod 0755 "$work/hello"
    archive="hello-linux-$arch.tar.gz"
    tar -C "$work" -czf "$work/$archive" hello
    (cd "$work" && sha256sum "$archive" > SHA256SUMS)
    for f in "$archive" SHA256SUMS; do
        curl -fsS -H "Authorization: token $token" -F "attachment=@$work/$f" \
            "$gitea/api/v1/repos/$repo/releases/$release_id/assets?name=$f" >/dev/null
    done
    rm -rf "$work"

    printf 'E2E_GITEA_TOKEN=%s\n' "$token" > /seed/gitea.env
fi

touch /seed/ready
