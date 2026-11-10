# Build Stage
FROM golang:1.27.3-bookworm AS builder

# Bound compiler concurrency on small deployment hosts.
ENV GOMAXPROCS=2 GOFLAGS=-p=2
ARG GO_JUDGE_VERSION=v1.14.0
RUN go install github.com/criyle/go-judge/cmd/go-judge@${GO_JUDGE_VERSION}

# Runtime Stage
FROM debian:bookworm-slim

# 避免交互
ENV DEBIAN_FRONTEND=noninteractive

# Judge compiler toolchains and health checks.
RUN apt-get update && apt-get install -y \
    g++ \
    python3 \
    openjdk-17-jdk \
    ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# Debian links JVM configuration into /etc. The judge mounts /usr read-only
# but intentionally does not expose all of /etc to submitted programs.
RUN java_home="$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")" && \
    cp -aL "$java_home/conf" /tmp/patreonoj-java-conf && \
    cp -a --remove-destination /tmp/patreonoj-java-conf/. "$java_home/conf/" && \
    rm -rf /tmp/patreonoj-java-conf

# Copy go-judge binary from builder
COPY --from=builder /go/bin/go-judge /usr/bin/go-judge
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

EXPOSE 5050

ENTRYPOINT ["/usr/bin/go-judge"]
CMD ["-http-addr", ":5050"]
