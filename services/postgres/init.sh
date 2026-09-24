#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -v env_pw="$E2E_PG_ENV_PW" -v stdin_pw="$E2E_PG_STDIN_PW" -v file_pw="$E2E_PG_FILE_PW" <<'SQL'
CREATE ROLE app_env LOGIN PASSWORD :'env_pw';
CREATE ROLE app_stdin LOGIN PASSWORD :'stdin_pw';
CREATE ROLE app_file LOGIN PASSWORD :'file_pw';
SQL
