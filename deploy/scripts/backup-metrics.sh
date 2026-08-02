#!/usr/bin/env bash
# Emits ZaaS backup metrics for the node_exporter textfile collector.
#
# Invoked by systemd as ExecStopPost= from zaas-backup.service and
# zaas-backup-volumes.service, so it runs on both success and failure - and also
# when a backup is killed, times out or is OOM-killed, which a trap inside the
# backup script itself could not catch.
#
# Usage: backup-metrics.sh <name> <directory> <glob>
#   name       value of the "backup" metric label, e.g. postgres
#   directory  directory holding the archives the run produced
#   glob       filename pattern used to total up their size
#
# The label is "backup" rather than "job": Prometheus's own scrape job label
# wins, and a job label in a textfile metric is silently renamed exported_job.
#
# systemd sets SERVICE_RESULT for ExecStopPost; anything other than "success"
# counts as a failed run. A failing ExecStopPost marks the whole unit failed, so
# this script must always exit 0 - hence -u and pipefail but deliberately no -e.

set -uo pipefail

# node_exporter runs as its own user and only needs to read these files, which
# hold timestamps and byte counts rather than anything sensitive.
umask 022

TEXTFILE_DIR="${TEXTFILE_DIR:-/var/lib/node_exporter/textfile_collector}"

NAME="${1:-}"
DIR="${2:-}"
GLOB="${3:-}"

if [ -z "${NAME}" ] || [ -z "${DIR}" ] || [ -z "${GLOB}" ]; then
    echo "usage: $(basename "$0") <name> <directory> <glob>" >&2
    exit 0
fi

if [ ! -d "${TEXTFILE_DIR}" ]; then
    echo "[$(date -Iseconds)] ${TEXTFILE_DIR} does not exist - no backup metrics written" >&2
    exit 0
fi

PROM_FILE="${TEXTFILE_DIR}/zaas-backup-${NAME}.prom"
TMP_FILE="${PROM_FILE}.$$"
NOW=$(date +%s)

trap 'rm -f "${TMP_FILE}"' EXIT

# SERVICE_RESULT is only set when systemd runs this as ExecStopPost; default to
# success so the script can be exercised by hand.
if [ "${SERVICE_RESULT:-success}" = "success" ]; then
    SUCCESS=1
else
    SUCCESS=0
fi

if [ "${SUCCESS}" -eq 1 ]; then
    LAST_SUCCESS="${NOW}"
    LAST_BYTES=$(find "${DIR}" -maxdepth 1 -type f -name "${GLOB}" -newermt '-24 hours' -printf '%s\n' 2>/dev/null \
        | awk '{ total += $1 } END { print total + 0 }')
else
    # Carry the previous values forward so a failed run does not erase the record
    # of the last good backup. With no previous file both stay unset, and the
    # ZaasBackupMetricsMissing absent() rule fires instead.
    LAST_SUCCESS=$(awk '/^zaas_backup_last_success_timestamp_seconds\{/ { print $2 }' "${PROM_FILE}" 2>/dev/null)
    LAST_BYTES=$(awk '/^zaas_backup_last_success_bytes\{/ { print $2 }' "${PROM_FILE}" 2>/dev/null)
fi

# Both backup jobs are written by this one script so that the HELP and TYPE text
# for a metric family is identical across files. Conflicting HELP text for the
# same family sets node_textfile_scrape_error and drops every textfile metric.
{
    echo "# HELP zaas_backup_last_run_timestamp_seconds Unix time of the last ZaaS backup run, successful or not."
    echo "# TYPE zaas_backup_last_run_timestamp_seconds gauge"
    echo "zaas_backup_last_run_timestamp_seconds{backup=\"${NAME}\"} ${NOW}"
    echo "# HELP zaas_backup_last_run_success Whether the last ZaaS backup run succeeded (1) or failed (0)."
    echo "# TYPE zaas_backup_last_run_success gauge"
    echo "zaas_backup_last_run_success{backup=\"${NAME}\"} ${SUCCESS}"

    if [ -n "${LAST_SUCCESS}" ]; then
        echo "# HELP zaas_backup_last_success_timestamp_seconds Unix time of the last successful ZaaS backup run."
        echo "# TYPE zaas_backup_last_success_timestamp_seconds gauge"
        echo "zaas_backup_last_success_timestamp_seconds{backup=\"${NAME}\"} ${LAST_SUCCESS}"
    fi

    if [ -n "${LAST_BYTES}" ]; then
        echo "# HELP zaas_backup_last_success_bytes Total bytes written by the last successful ZaaS backup run."
        echo "# TYPE zaas_backup_last_success_bytes gauge"
        echo "zaas_backup_last_success_bytes{backup=\"${NAME}\"} ${LAST_BYTES}"
    fi
} > "${TMP_FILE}"

# node_exporter re-reads the directory on every scrape, so swap the file in
# atomically rather than letting a partial write be scraped.
if [ -s "${TMP_FILE}" ]; then
    mv "${TMP_FILE}" "${PROM_FILE}"
    echo "[$(date -Iseconds)] Backup metrics written: ${PROM_FILE} (success=${SUCCESS})"
else
    echo "[$(date -Iseconds)] Failed to write ${PROM_FILE}" >&2
fi

exit 0
