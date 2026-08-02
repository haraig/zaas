#!/usr/bin/env bash
# Docker volume GFS backup for ZaaS.
# Runs daily via systemd timer. Stores compressed tar archives with
# Grandfather-Father-Son rotation:
#   - 7 daily backups
#   - 4 weekly backups (Sundays)
#   - 6 monthly backups (first Sunday of month)
#
# Backup directory: /var/backups/zaas/volumes/ (owned by deploy:deploy)
#
# Covered volumes hold state that cannot be rebuilt from git:
#   - caddy_data        Let's Encrypt account key and issued certificates
#   - grafana_data      users, API keys, UI-created dashboards, annotations
#   - alertmanager_data active silences
#
# postgres_data is deliberately not covered here: a file-level copy of a running
# data directory is not crash-consistent. The logical pg_dump in
# backup-postgres.sh is the correct tool for it.

set -euo pipefail
umask 027

BACKUP_DIR="${BACKUP_DIR:-/var/backups/zaas}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/zaas/deploy/docker-compose.yaml}"
ENV_FILE="${ENV_FILE:-/opt/zaas/.env}"

# Pinned GNU tar image, consistent with how third-party images are pinned in
# docker-compose.yaml. BusyBox tar (i.e. any alpine image) does not support
# --numeric-owner, which is required because Grafana runs as uid 472 and
# Alertmanager as uid 65534 - names that do not resolve inside the helper.
TAR_IMAGE="${TAR_IMAGE:-debian:13-slim@sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd}"

# Compose prefixes volume names with the project name, which defaults to the
# compose file's directory ("deploy") - the same assumption behind the
# deploy-postgres-1 container name in backup-postgres.sh.
VOLUMES="deploy_caddy_data deploy_grafana_data deploy_alertmanager_data"

DATE=$(date +%Y-%m-%d)
DOW=$(date +%u)  # 1=Monday, 7=Sunday
DOM=$(date +%d)  # Day of month (zero-padded, e.g. 07)

VOLUMES_DIR="${BACKUP_DIR}/volumes"
DAILY_DIR="${VOLUMES_DIR}/daily"
WEEKLY_DIR="${VOLUMES_DIR}/weekly"
MONTHLY_DIR="${VOLUMES_DIR}/monthly"

mkdir -p "${DAILY_DIR}" "${WEEKLY_DIR}" "${MONTHLY_DIR}"

# Services that must be stopped for a consistent copy of their volume.
# grafana_data holds grafana.db, a live SQLite file, and a hot tar can capture a
# torn write. caddy_data and alertmanager_data are written atomically and are
# safe to tar while running.
service_for_volume() {
    case "$1" in
        deploy_grafana_data) echo "grafana" ;;
        *) echo "" ;;
    esac
}

# Paths left out of a volume's archive. Grafana keeps its default plugin set in
# the data volume - ~85 MB of re-fetchable code that it reinstalls on startup.
# The backup exists for grafana.db (users, API keys, UI-created dashboards,
# annotations), which is around 1.5 MB.
excludes_for_volume() {
    case "$1" in
        deploy_grafana_data) echo "--exclude=./plugins" ;;
        *) echo "" ;;
    esac
}

compose() {
    docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" "$@"
}

STOPPED_SERVICE=""
CURRENT_TMP=""

start_stopped_service() {
    if [ -n "${STOPPED_SERVICE}" ]; then
        echo "[$(date -Iseconds)] Starting ${STOPPED_SERVICE}..."
        compose start "${STOPPED_SERVICE}" || true
        STOPPED_SERVICE=""
    fi
}

# Runs on any exit path, so a failed tar never leaves a stopped service or a
# half-written archive behind.
cleanup() {
    rm -f "${CURRENT_TMP:-}"
    start_stopped_service
}
trap cleanup EXIT

echo "[$(date -Iseconds)] Starting Docker volume backup..."

for vol in ${VOLUMES}; do
    archive="${DAILY_DIR}/${vol}-${DATE}.tar.gz"
    CURRENT_TMP="${archive}.tmp"
    service=$(service_for_volume "${vol}")

    read -r -a excludes <<< "$(excludes_for_volume "${vol}")"

    if [ -n "${service}" ]; then
        echo "[$(date -Iseconds)] Stopping ${service} for a consistent copy of ${vol}..."
        compose stop "${service}"
        STOPPED_SERVICE="${service}"
    fi

    # The helper container runs as root so tar can read files owned by the
    # service uids, but writes to stdout - so the archive itself is created by
    # this script under umask 027 as deploy:deploy, not as root.
    docker run --rm \
        -v "${vol}:/source:ro" \
        "${TAR_IMAGE}" \
        tar czf - --numeric-owner "${excludes[@]}" -C /source . > "${CURRENT_TMP}"

    start_stopped_service

    # Verify before moving into place, so a failed run never leaves a truncated
    # archive that looks like a valid backup.
    gzip -t "${CURRENT_TMP}"
    mv "${CURRENT_TMP}" "${archive}"
    CURRENT_TMP=""
    echo "[$(date -Iseconds)] Daily backup created: ${archive}"

    # Promote to weekly on Sundays (DOW=7)
    if [ "${DOW}" -eq 7 ]; then
        cp "${archive}" "${WEEKLY_DIR}/${vol}-${DATE}.tar.gz"
        echo "[$(date -Iseconds)] Weekly backup promoted: ${vol}"

        # Promote to monthly on first Sunday (day 1-7)
        if [ "${DOM}" -le 7 ]; then
            cp "${archive}" "${MONTHLY_DIR}/${vol}-${DATE}.tar.gz"
            echo "[$(date -Iseconds)] Monthly backup promoted: ${vol}"
        fi
    fi
done

# Rotate: keep 7 daily, 4 weekly, 6 monthly
find "${DAILY_DIR}" -name "*.tar.gz" -mtime +7 -delete
find "${WEEKLY_DIR}" -name "*.tar.gz" -mtime +28 -delete
find "${MONTHLY_DIR}" -name "*.tar.gz" -mtime +180 -delete

# Sweep up temporary files orphaned by an interrupted run (e.g. a reboot).
find "${DAILY_DIR}" -name "*.tar.gz.tmp" -mtime +1 -delete

echo "[$(date -Iseconds)] Backup complete. Rotation applied."
