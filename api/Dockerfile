# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.26.3-alpine3.23@sha256:f44b851aa23dfa219d18db6eab743203245429d355cb619cf96a2ffe2a84ba7a AS builder
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
