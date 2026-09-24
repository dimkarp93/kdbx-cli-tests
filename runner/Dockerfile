FROM golang:1.26-trixie
ARG INSTALL_REF=c4e7d48323902b19531b60a466038b228544d393
ARG KDBX_CLI_VERSION
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      keepassxc-minimal postgresql-client mariadb-client openssh-client git restic gnupg \
      skopeo sudo curl jq procps util-linux netcat-openbsd openssl ca-certificates \
 && rm -rf /var/lib/apt/lists/*
ADD --chmod=0755 https://raw.githubusercontent.com/dimkarp93/install/${INSTALL_REF}/github_install.sh /opt/install/github_install.sh
ADD --chmod=0755 https://raw.githubusercontent.com/dimkarp93/install/${INSTALL_REF}/gitea_install.sh /opt/install/gitea_install.sh
ADD --chmod=0755 https://raw.githubusercontent.com/dimkarp93/install/${INSTALL_REF}/gitlab_install.sh /opt/install/gitlab_install.sh
RUN test -n "${KDBX_CLI_VERSION}" \
 && /opt/install/github_install.sh -s https://github.com dimkarp93/kdbx-cli kdbx-cli "${KDBX_CLI_VERSION}" \
 && test "$(kdbx-cli --version)" = "${KDBX_CLI_VERSION}"
RUN useradd -m -s /bin/bash tester \
 && printf 'Defaults timestamp_timeout=0\ntester ALL=(ALL) ALL\n' > /etc/sudoers.d/tester \
 && chmod 0440 /etc/sudoers.d/tester \
 && install -d -o tester -g tester /work /seed
COPY --chown=tester:tester . /src
COPY --chmod=0755 runner/entrypoint.sh /usr/local/bin/runner-entrypoint
ENV GOWORK=off GOFLAGS=-mod=vendor GOCACHE=/tmp/gocache GOTOOLCHAIN=local \
    KDBX_CLI_BIN=/usr/local/bin/kdbx-cli KDBX_CLI_E2E_ROOT=/work E2E_IN_DOCKER=1
WORKDIR /src
ENTRYPOINT ["/usr/local/bin/runner-entrypoint"]
CMD ["bash"]
