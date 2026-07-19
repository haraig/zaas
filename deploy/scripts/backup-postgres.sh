#!/usr/bin/env bash
# PostgreSQL GFS backup for ZaaS.
# Runs daily via systemd timer. Stores compressed SQL dumps with
# Grandfather-Father-Son rotation:
#   - 7 daily backups
#   - 4 weekly backups (Sundays)
#   - 6 monthly backups (first Sunday of month)
#
# Backup directory: /var/backups/zaas/ (owned by deploy:deploy)
# Container name: zaas-postgres-1

set -euo pipefail

BACKUP_DIR="/var/backups/zaas"
CONTAINER="zaas-postgres-1"
DATE=$(date +%Y-%m-%d)
DOW=$(date +%u)  # 1=Monday, 7=Sunday
DOM=$(date +%d)  # Day of month (zero-padded, e.g. 07)

DAILY_DIR="${BACKUP_DIR}/daily"
WEEKLY_DIR="${BACKUP_DIR}/weekly"
MONTHLY_DIR="${BACKUP_DIR}/monthly"

mkdir -p "${DAILY_DIR}" "${WEEKLY_DIR}" "${MONTHLY_DIR}"

DUMP_FILE="${DAILY_DIR}/zaas-${DATE}.sql.gz"

echo "[$(date -Iseconds)] Starting PostgreSQL backup..."

# Dump database
docker exec "${CONTAINER}" pg_dump -U zaas zaas | gzip > "${DUMP_FILE}"

echo "[$(date -Iseconds)] Daily backup created: ${DUMP_FILE}"

# Promote to weekly on Sundays (DOW=7)
if [ "${DOW}" -eq 7 ]; then
    cp "${DUMP_FILE}" "${WEEKLY_DIR}/zaas-${DATE}.sql.gz"
    echo "[$(date -Iseconds)] Weekly backup promoted"

    # Promote to monthly on first Sunday (day 1-7)
    if [ "${DOM}" -le 7 ]; then
        cp "${DUMP_FILE}" "${MONTHLY_DIR}/zaas-${DATE}.sql.gz"
        echo "[$(date -Iseconds)] Monthly backup promoted"
    fi
fi

# Rotate: keep 7 daily, 4 weekly, 6 monthly
find "${DAILY_DIR}" -name "zaas-*.sql.gz" -mtime +7 -delete
find "${WEEKLY_DIR}" -name "zaas-*.sql.gz" -mtime +28 -delete
find "${MONTHLY_DIR}" -name "zaas-*.sql.gz" -mtime +180 -delete

echo "[$(date -Iseconds)] Backup complete. Rotation applied."
