#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -v stdin_pw="$E2E_PG_STDIN_PW" <<'SQL'
CREATE ROLE app_stdin LOGIN PASSWORD :'stdin_pw';
SQL
