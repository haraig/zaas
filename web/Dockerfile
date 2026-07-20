# syntax=docker/dockerfile:1

# --- Build stage: Astro static site ---
FROM node:26-alpine@sha256:e88a35be04478413b7c71c455cd9865de9b9360e1f43456be5951032d7ac1a66 AS builder
WORKDIR /app

# Build-time env vars injected by CI (used by Astro's import.meta.env.PUBLIC_*)
ARG PUBLIC_API_BASE_URL
ARG PUBLIC_IMPRINT_NAME
ARG PUBLIC_IMPRINT_ADDRESS
ARG PUBLIC_IMPRINT_EMAIL
ARG PUBLIC_PRIVACY_EMAIL

ENV PUBLIC_API_BASE_URL=$PUBLIC_API_BASE_URL
ENV PUBLIC_IMPRINT_NAME=$PUBLIC_IMPRINT_NAME
ENV PUBLIC_IMPRINT_ADDRESS=$PUBLIC_IMPRINT_ADDRESS
ENV PUBLIC_IMPRINT_EMAIL=$PUBLIC_IMPRINT_EMAIL
ENV PUBLIC_PRIVACY_EMAIL=$PUBLIC_PRIVACY_EMAIL

COPY package.json package-lock.json ./
RUN npm ci

# Drop root before touching source files so all build artifacts (including
# the Vite cache under node_modules/.vite) are owned by the app user.
RUN addgroup -S app && adduser -S app -G app && chown -R app:app /app
USER app

COPY --chown=app:app . .
RUN ASTRO_TELEMETRY_DISABLED=1 npm run build

# --- Runtime stage: Caddy with static assets ---
FROM caddy:2.11.4-alpine@sha256:5f5c8640aae01df9654968d946d8f1a56c497f1dd5c5cda4cf95ab7c14d58648
COPY --from=builder /app/dist /srv/web
