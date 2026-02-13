# Build stage
FROM golang:1.25.1 AS builder

COPY go.mod go.sum /app/

# Set working directory
WORKDIR /app

ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache

# Copy only Go source files
COPY main.go ./
COPY cmd/ ./cmd/

ARG TARGETOS
ARG TARGETARCH
ARG release=
RUN --mount=type=cache,target=/go-cache \
  --mount=type=cache,target=/gomod-cache \
  <<EOR
  VERSION=$(git rev-parse --short HEAD)
  BUILDTIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  RELEASE=$release
  CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags=ckzg -o /app/ -ldflags="-s -w" .
EOR

# Final stage - using alpine for shell access
FROM debian:stable-slim

# Set working directory
WORKDIR /app

# Install basic tools for debugging
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

# Set PATH
ENV PATH="$PATH:/app"

# Copy the binary
COPY --from=builder /app/consensus-proxy /consensus-proxy

# Expose port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/consensus-proxy"]