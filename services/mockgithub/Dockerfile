FROM golang:1.26-trixie AS build
WORKDIR /src
COPY . .
ENV GOWORK=off GOFLAGS=-mod=vendor CGO_ENABLED=0
RUN go build -trimpath -o /out/mockgithub ./services/mockgithub

FROM busybox:1.37
COPY --from=build /out/mockgithub /usr/local/bin/mockgithub
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=3s --retries=30 CMD wget -qO- http://127.0.0.1:8080/__health || exit 1
ENTRYPOINT ["/usr/local/bin/mockgithub"]
