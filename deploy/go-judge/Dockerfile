# Build Stage
FROM golang:latest AS builder

# 使用代理加速下载
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go install github.com/criyle/go-judge/cmd/go-judge@latest

# Runtime Stage
FROM ubuntu:22.04

# 避免交互
ENV DEBIAN_FRONTEND=noninteractive

# Use Aliyun mirror for faster apt installs
RUN sed -i s@/archive.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list
RUN sed -i s@/security.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list

# Install compilers (GCC, G++, Python3, Java 17)
# Removed golang-go because runtime doesn't strictly need Go compiler unless we judge Go code. 
# But wait, user wants to judge Go code? Yes.
# So we should install golang-go in runtime or copy /usr/local/go from builder?
# Installing via apt is easier for runtime dependency management.
RUN apt-get update && apt-get install -y \
    g++ \
    python3 \
    openjdk-17-jdk \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

# Copy go-judge binary from builder
COPY --from=builder /go/bin/go-judge /usr/bin/go-judge

EXPOSE 5050

ENTRYPOINT ["/usr/bin/go-judge", "-http-addr", ":5050"]
