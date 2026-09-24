#!/bin/sh
set -eu
echo "tester:${E2E_SUDO_PW:?}" | chpasswd
chown tester:tester /seed
export HOME=/home/tester USER=tester LOGNAME=tester
exec setpriv --reuid tester --regid tester --init-groups -- "$@"
