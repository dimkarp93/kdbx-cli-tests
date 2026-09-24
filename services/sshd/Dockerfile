FROM alpine:3.22
RUN apk add --no-cache openssh-server \
 && adduser -D -s /bin/sh alice
COPY sshd_config /etc/ssh/sshd_config
COPY --chmod=0755 entrypoint.sh /usr/local/bin/sshd-entrypoint
EXPOSE 22
HEALTHCHECK --interval=2s --timeout=3s --retries=30 CMD nc -z 127.0.0.1 22 || exit 1
ENTRYPOINT ["/usr/local/bin/sshd-entrypoint"]
