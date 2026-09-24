#!/bin/sh
set -eu
ssh-keygen -A >/dev/null
echo "alice:${E2E_SSH_PW:?}" | chpasswd
mkdir -p /seed/ssh
touch /seed/ssh/authorized_keys
chmod 0777 /seed/ssh
chmod 0666 /seed/ssh/authorized_keys
exec /usr/sbin/sshd -D -e
