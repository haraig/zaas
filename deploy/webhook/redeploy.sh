#!/bin/sh
set -e
cd /opt/zaas
# Allow git to operate on a directory owned by a different UID (webhook runs as root).
git config --global --add safe.directory /opt/zaas
git pull --ff-only
docker compose -f deploy/docker-compose.yaml pull api caddy
docker compose -f deploy/docker-compose.yaml up -d --no-deps api caddy
