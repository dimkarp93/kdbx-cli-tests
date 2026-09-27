#!/usr/bin/env bash
set -euo pipefail

store="/tmp/demo-store.kdbx"
password="demo-pass-123"
pg_demo_password="demo-pg-pass-123"
gpg_passphrase="demo-gpg-pass-123"
gnupg_home="/tmp/demo-gnupg"
secret_plain="/tmp/demo-secret.txt"
secret_enc="${secret_plain}.gpg"
rm -f "$store"

PGPASSWORD="$E2E_PG_ADMIN_PW" psql -h postgres -U postgres -d postgres -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_demo') THEN
      CREATE ROLE app_demo LOGIN PASSWORD '$pg_demo_password';
   END IF;
END
\$\$;
SQL

rm -rf "$gnupg_home"
mkdir -m 700 -p "$gnupg_home"
export GNUPGHOME="$gnupg_home"
printf 'top secret build token\n' > "$secret_plain"
rm -f "$secret_enc"
gpg --batch --pinentry-mode loopback --passphrase "$gpg_passphrase" --symmetric -o "$secret_enc" "$secret_plain"
gpgconf --kill gpg-agent || true
rm -f "$secret_plain"

xml="$(mktemp)"
cat > "$xml" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<KeePassFile><Root><Group><Name>Root</Name>
<Entry>
  <String><Key>Title</Key><Value>MARK</Value></String>
  <String><Key>Password</Key><Value>marker-value</Value></String>
</Entry>
<Entry>
  <String><Key>Title</Key><Value>ssh-pw</Value></String>
  <String><Key>Password</Key><Value>${E2E_SSH_PW}</Value></String>
</Entry>
<Entry>
  <String><Key>Title</Key><Value>pg-stdin</Value></String>
  <String><Key>Password</Key><Value>${pg_demo_password}</Value></String>
</Entry>
<Entry>
  <String><Key>Title</Key><Value>gpg-pass</Value></String>
  <String><Key>Password</Key><Value>${gpg_passphrase}</Value></String>
</Entry>
</Group></Root></KeePassFile>
EOF

printf '%s\n%s\n' "$password" "$password" | keepassxc-cli import -q -p "$xml" "$store"
rm -f "$xml"
