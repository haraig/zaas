# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.27.3-alpine3.24@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder
WORKDIR /app

# VERSION is injected by `make build` from git describe.
ARG VERSION=dev

# Download dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o /zaas-api ./cmd/server

# --- Runtime stage ---
FROM scratch
COPY --from=builder /zaas-api /zaas-api

# OTel standard env defaults (overridden by docker-compose / K8s)
ENV OTEL_SERVICE_NAME=zaas-api

EXPOSE 8080

ENTRYPOINT ["/zaas-api"]
