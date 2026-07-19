# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder
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
