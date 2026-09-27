FROM golang:1.26-trixie
ARG INSTALL_REF=c4e7d48323902b19531b60a466038b228544d393
ARG KDBX_CLI_VERSION
ARG KDBX_CLI_LOCAL=0
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      keepassxc-minimal postgresql-client openssh-client git restic gnupg \
      sudo curl jq procps util-linux netcat-openbsd openssl ca-certificates \
      ffmpeg chromium \
 && rm -rf /var/lib/apt/lists/*
ARG TTYD_VERSION=1.7.7
RUN arch="$(dpkg --print-architecture)" \
 && case "$arch" in amd64) ttyd_arch=x86_64 ;; arm64) ttyd_arch=aarch64 ;; *) echo "unsupported arch: $arch" >&2; exit 1 ;; esac \
 && curl -fsSL -o /usr/local/bin/ttyd "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${ttyd_arch}" \
 && chmod 0755 /usr/local/bin/ttyd
ADD --chmod=0755 https://raw.githubusercontent.com/dimkarp93/install/${INSTALL_REF}/github_install.sh /opt/install/github_install.sh
COPY runner/kdbx-cli-src /opt/kdbx-cli-src
RUN test -n "${KDBX_CLI_VERSION}" \
 && if [ "${KDBX_CLI_LOCAL}" = "1" ]; then \
      cd /opt/kdbx-cli-src \
      && GOWORK=off GOFLAGS=-mod=vendor CGO_ENABLED=0 go build -trimpath \
           -ldflags="-s -w -X main.version=${KDBX_CLI_VERSION} -X main.channel=local" \
           -o /usr/local/bin/kdbx-cli ./cmd/kdbx-cli; \
    else \
      /opt/install/github_install.sh -s https://github.com dimkarp93/kdbx-cli kdbx-cli "${KDBX_CLI_VERSION}"; \
    fi \
 && test "$(kdbx-cli --version)" = "${KDBX_CLI_VERSION}"
RUN go install github.com/charmbracelet/vhs@latest \
 && mv "$(go env GOPATH)/bin/vhs" /usr/local/bin/vhs
RUN useradd -m -s /bin/bash tester \
 && printf 'Defaults timestamp_timeout=0\ntester ALL=(ALL) ALL\n' > /etc/sudoers.d/tester \
 && chmod 0440 /etc/sudoers.d/tester \
 && install -d -o tester -g tester /work /seed
COPY --chown=tester:tester . /src
COPY --chmod=0755 runner/entrypoint.sh /usr/local/bin/runner-entrypoint
ENV GOWORK=off GOFLAGS=-mod=vendor GOCACHE=/tmp/gocache GOTOOLCHAIN=local \
    KDBX_CLI_BIN=/usr/local/bin/kdbx-cli KDBX_CLI_E2E_ROOT=/work E2E_IN_DOCKER=1 \
    BROWSER=/usr/bin/chromium VHS_NO_SANDBOX=true
WORKDIR /src
ENTRYPOINT ["/usr/local/bin/runner-entrypoint"]
CMD ["bash"]
