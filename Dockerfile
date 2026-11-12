ARG GO_IMAGE=golang:1.27.3-bookworm
FROM ${GO_IMAGE} AS build
WORKDIR /src
ENV CGO_ENABLED=1 GOMAXPROCS=2
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build go test -p 2 ./... && \
    go build -p 2 -trimpath -ldflags="-s -w" -o /out/patreonoj ./cmd/PatreonOJ && \
    go build -p 2 -trimpath -ldflags="-s -w" -o /out/patreonoj-admin ./cmd/admin && \
    go build -p 2 -trimpath -ldflags="-s -w" -o /out/patreonoj-graph ./cmd/rebuild_graph

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --gid 10001 app && useradd --uid 10001 --gid app --no-create-home app && \
    install -d -o app -g app /app/data
WORKDIR /app
COPY --from=build /out/ /app/
COPY --chown=10001:10001 --chmod=640 deploy/config.compose.yaml /app/config.yaml
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=30s --retries=5 \
  CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/patreonoj"]
