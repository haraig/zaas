#!/usr/bin/env bash
# ============================================================================
# THIS SCRIPT RUNS ON THE BACKUP SERVER, NOT ON THE ZAAS SERVER.
# ============================================================================
# It lives in this repository for version control, and the ZaaS server does get
# a copy through the sparse checkout of deploy/ into /opt/zaas - but running it
# there does nothing useful.
#
# Pulls /var/backups/zaas from the ZaaS server over SSH. The reader account on
# the far side is confined by a forced command to reading that one directory:
#
#   restrict,command="/usr/bin/rrsync -ro /var/backups/zaas" ssh-ed25519 AAAA...
#
# so the ZaaS server holds no outbound credentials and cannot reach, modify or
# delete the offsite copies. See docs/reference/runbook.md for the account setup.
#
# Configuration is by environment variable so no hostname is baked in:
#   ZAAS_BACKUP_HOST  hostname or IP of the ZaaS server (required)
#   ZAAS_BACKUP_USER  reader account          (default: zaasbackup)
#   ZAAS_BACKUP_PORT  SSH port                (default: 2222)
#   ZAAS_BACKUP_KEY   private key             (default: ~/.ssh/zaas_backup)
#   ZAAS_BACKUP_DEST  local destination       (default: /var/backups/zaas-offsite)

set -euo pipefail

HOST="${ZAAS_BACKUP_HOST:-}"
USER="${ZAAS_BACKUP_USER:-zaasbackup}"
PORT="${ZAAS_BACKUP_PORT:-2222}"
KEY="${ZAAS_BACKUP_KEY:-${HOME}/.ssh/zaas_backup}"
DEST="${ZAAS_BACKUP_DEST:-/var/backups/zaas-offsite}"

if [ -z "${HOST}" ]; then
    echo "ZAAS_BACKUP_HOST is not set." >&2
    echo "usage: ZAAS_BACKUP_HOST=<host> $(basename "$0")" >&2
    exit 1
fi

if [ ! -r "${KEY}" ]; then
    echo "SSH key ${KEY} is missing or unreadable." >&2
    exit 1
fi

mkdir -p "${DEST}"

echo "[$(date -Iseconds)] Pulling backups from ${USER}@${HOST}:${PORT} into ${DEST}..."

# The source is ":/" rather than an absolute path: with rrsync, remote paths are
# relative to the locked root, so ":/" already means /var/backups/zaas. Passing
# the absolute host path fails.
#
# --delete is deliberately omitted. Source-side GFS rotation would otherwise
# propagate here and delete exactly the history this copy exists to preserve.
# StrictHostKeyChecking stays on: an occasional manual command is precisely the
# case where a silent host-key swap would go unnoticed.
rsync -az --info=stats2 --partial \
    -e "ssh -p ${PORT} -i ${KEY} -o StrictHostKeyChecking=yes" \
    "${USER}@${HOST}:/" "${DEST}/"

echo "[$(date -Iseconds)] Pull complete. Offsite copy:"
du -sh "${DEST}"
