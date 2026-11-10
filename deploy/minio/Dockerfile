ARG GO_IMAGE=golang:1.27.3-bookworm
FROM ${GO_IMAGE} AS build
ARG MINIO_VERSION=RELEASE.2025-10-15T17-29-55Z
ENV CGO_ENABLED=0 GOMAXPROCS=2 GOFLAGS=-p=2
RUN go install -trimpath -ldflags="-s -w" github.com/minio/minio@${MINIO_VERSION}

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --gid 10002 minio && useradd --uid 10002 --gid minio --no-create-home minio && \
    install -d -o minio -g minio /data
COPY --from=build /go/bin/minio /usr/local/bin/minio
USER 10002:10002
EXPOSE 9000
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=5 \
  CMD curl -fsS http://127.0.0.1:9000/minio/health/live || exit 1
ENTRYPOINT ["/usr/local/bin/minio"]
