# ZaaS Operations Runbook

Manual steps and operational procedures that cannot be automated.

> **AI agents:** When writing implementation plans that include manual steps, reference this document instead of embedding the steps in the plan. Add new entries here as part of the plan.

---

## Table of Contents

### Part 1: Server Setup Sequence

Follow these steps in order when setting up a new server from scratch.

1. [Initial Infrastructure Setup](#1-initial-infrastructure-setup)
2. [Server Bootstrap](#2-server-bootstrap)
3. [GitHub Actions Configuration](#3-github-actions-configuration)
4. [Email: Migadu Mailbox Setup](#4-email-migadu-mailbox-setup)
5. [Email: DNS Records Setup](#5-email-dns-records-setup)
6. [Email: Production .env Configuration](#6-email-production-env-configuration)
7. [Email: End-to-End Verification](#7-email-end-to-end-verification)
8. [PostgreSQL: Production .env Configuration](#8-postgresql-production-env-configuration)
9. [Backups: Timers and Metrics](#9-backups-timers-and-metrics)
10. [Redis: Production .env Configuration](#10-redis-production-env-configuration)
11. [Node Exporter: Host Metrics](#11-node-exporter-host-metrics)
12. [Offsite Backups: Read-Only Reader Account](#12-offsite-backups-read-only-reader-account)
13. [Slack Alert Notifications](#13-slack-alert-notifications)

### Part 2: Operational Procedures

Ad-hoc and ongoing procedures, looked up as needed.

- [Restarting the Server](#restarting-the-server)
- [Releases](#releases)
- [DMARC Policy Tightening](#dmarc-policy-tightening)
- [Issuing an API Key on Request](#issuing-an-api-key-on-request)
- [Accessing the API Container Directly](#accessing-the-api-container-directly)
- [PostgreSQL: Manual Database Access](#postgresql-manual-database-access)
- [Client Administration (SQL Reference)](#client-administration-sql-reference)
- [Offsite Backups: Manual Pull](#offsite-backups-manual-pull)
- [Accessing Internal Service UIs](#accessing-internal-service-uis)

### Part 3: Alert Playbooks

Procedures for responding to Prometheus alerts. Each section corresponds to an alert rule in `deploy/prometheus.rules.yaml`.

- [First Response Checklist](#first-response-checklist)
- [ZaasApiDown](#zaasapidown)
- [ZaasHighErrorRate](#zaashigherrorrate)
- [ZaasHighP99Latency](#zaashighp99latency)
- [ZaasHighRateLimitRate](#zaashighratelimitrate)
- [ZaasDiskSpaceLow / ZaasDiskSpaceCritical](#zaasdiskspacelow)
- [ZaasPostgresDown](#zaaspostgresdown)
- [ZaasRedisDown](#zaasredisdown)
- [ZaasCollectorDroppedSpans / ZaasCollectorDroppedMetrics / ZaasCollectorDroppedLogs / ZaasCollectorDown](#zaascollectordroppeddata)
- [ZaasBackupFailed / ZaasBackupStale / ZaasBackupMetricsMissing](#zaasbackupstale)
- [ZaasWatchdog](#zaaswatchdog)
- [PostgreSQL: Restore from pg_dump](#postgresql-restore-from-pg_dump)
- [Docker Volumes: Restore from tar backup](#docker-volumes-restore-from-tar-backup)
- [Full Server Loss: Rebuild from Offsite](#full-server-loss-rebuild-from-offsite)
- [Observability Stack Bootstrap (data loss)](#observability-stack-bootstrap-data-loss)

---

# Part 1: Server Setup Sequence

## 1. Initial Infrastructure Setup

**Source:** `infra/tofu/` (see `docs/how-to/infrastructure.md`)

### 1.1 Prerequisites

- OpenTofu installed (`tofu` CLI)
- Hetzner Cloud API token
- SSH key pair (generate if needed: `ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_hetzner -C "hetzner" -N ""`)

### 1.2 Domain Delegation (at registrar)

If your domain is registered elsewhere, point it at Hetzner's nameservers:

| Nameserver |
|---|
| `hydrogen.ns.hetzner.com` |
| `oxygen.ns.hetzner.com` |
| `helium.ns.hetzner.de` |

Propagation can take 24-48 hours. Verify with:

```bash
dig NS zaas.at
```

### 1.3 Provision Infrastructure

```bash
cp infra/tofu/terraform.tfvars.example infra/tofu/terraform.tfvars
# Edit terraform.tfvars: set user_name, domain_tld, server_name

export TF_VAR_hcloud_token="your_token"   # prefix with a space to keep out of shell history
make infra-init
make infra-plan   # review before applying
make infra-apply
```

### 1.4 Firewall

`make infra-apply` also creates a Hetzner Cloud Firewall (`enable_firewall`, default `true`)
and attaches it to the server. It allows only inbound SSH (`ssh_port`), HTTP, HTTPS and ICMP;
everything else is dropped at Hetzner's edge.

This is the authoritative ingress control, **not** ufw. Docker publishes container ports
through the `FORWARD` chain, which never traverses the `INPUT` chain that ufw filters, so ufw
cannot restrict any published container port. The cloud firewall sits outside the host and is
not bypassable that way.

Two consequences:

- Exposing a new port publicly needs a rule here **and** the corresponding compose change.
  Adding only `ports:` in `deploy/docker-compose.yaml` silently does nothing.
- The firewall is deliberately **inbound-only**. A Hetzner firewall leaves egress
  unrestricted only while it has zero outbound rules; adding even one drops everything else
  outbound and would break ACME issuance, GHCR pulls, SMTP and alert notifications.

Verify from a machine outside the server:

```bash
for p in 9090 9093 3000 3100 3200; do
  echo -n "$p: "
  curl -sS --connect-timeout 5 -o /dev/null -w '%{http_code}\n' http://<server-ip>:$p/ \
    || echo "refused/filtered (expected)"
done

curl -sS https://<domain>/healthz   # must still work
```

Applying the firewall to an already-running server does not replace it - the attachment is a
separate resource and leaves `hcloud_server` untouched. Confirm `make infra-plan` reports
**0 to destroy** before applying, and keep an SSH session open while you do.

---

## 2. Server Bootstrap

**Source:** `docs/how-to/deploy.md`

The server is provisioned from the `ubuntu-26.04` image. Cloud-init (`infra/tofu/modules/hcloud_server/user_data.yaml.tftpl`) installs Docker from the official Docker CE apt repository, runs `package_update` and `package_upgrade`, disables root login, and reboots once provisioning finishes. SSH in as the configured user on the custom SSH port (default: `2222`):

```bash
ssh -p 2222 <user_name>@<server-ip>
```

Create the deploy user and application directory:

```bash
# Create deploy user
sudo useradd --system --no-create-home --shell /usr/sbin/nologin --groups docker deploy
sudo mkdir -p /opt/zaas
sudo chown deploy:deploy /opt/zaas

# Sparse-clone repo (deploy/ folder only)
sudo -u deploy sh -c '
  git clone --filter=blob:none --sparse https://github.com/haraig/zaas.git /opt/zaas
  git -C /opt/zaas sparse-checkout set deploy
'

# Create .env
sudo -u deploy sh -c '
  cp /opt/zaas/.env.example /opt/zaas/.env
  chmod 600 /opt/zaas/.env
'
```

The `chmod 600` restricts the file to the `deploy` user only, since it will hold secrets (`GRAFANA_ADMIN_PASSWORD`, `DEPLOY_WEBHOOK_SECRET`).

Edit `/opt/zaas/.env` and set at minimum:

```dotenv
ZAAS_DOMAIN=zaas.at
GRAFANA_ADMIN_PASSWORD=<strong-password>
DEPLOY_WEBHOOK_SECRET=<openssl rand -hex 32>
```

> `PUBLIC_API_BASE_URL`, `PUBLIC_IMPRINT_NAME`, `PUBLIC_IMPRINT_ADDRESS`, `PUBLIC_IMPRINT_EMAIL`, and
> `PUBLIC_PRIVACY_EMAIL` are **not** read from this file. The web image is a prebuilt static site -
> these values are baked in at CI build time from GitHub Actions variables (see Section 3.2 below).
> Setting them here has no effect in production.

Start the stack:

```bash
sudo -u deploy sh -c '
  cd /opt/zaas
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d
'
```

Unlike the other services, `webhook` has no `image:` to pull - it's built locally from `deploy/webhook/Dockerfile`, so this first `up -d` also builds it (needs outbound network access for `apk add`). Confirm it built with the tools `redeploy.sh` needs:

```bash
docker exec deploy-webhook-1 git --version
docker exec deploy-webhook-1 docker --version
```

Verify:

```bash
curl https://zaas.at/healthz
# -> {"status":"ok"}
```

---

## 3. GitHub Actions Configuration

**Source:** `docs/how-to/deploy.md`

### 3.1 Actions Secrets

Go to **Settings -> Secrets and variables -> Actions -> Secrets**:

| Secret | Description | How to generate |
|---|---|---|
| `DEPLOY_WEBHOOK_SECRET` | HMAC-SHA256 signing secret for deploy webhook | `openssl rand -hex 32` |

### 3.2 Actions Variables

Go to **Settings -> Secrets and variables -> Actions -> Variables**:

| Variable | Value |
|---|---|
| `PUBLIC_API_BASE_URL` | `https://zaas.at` |
| `PUBLIC_IMPRINT_NAME` | Legal name |
| `PUBLIC_IMPRINT_ADDRESS` | Postal address (use `\n` for line breaks) |
| `PUBLIC_IMPRINT_EMAIL` | `contact@zaas.at` |
| `PUBLIC_PRIVACY_EMAIL` | `privacy@zaas.at` |

### 3.3 Branch Protection

Go to **Settings -> Branches -> Add rule** for `main`:

1. Enable **Require status checks to pass before merging**
2. Add required checks: `Go - fmt / vet / lint / test`, `Web - lint + build`, `Docker - build smoke tests`

---

## 4. Email: Migadu Mailbox Setup

Create these mailboxes in the [Migadu admin panel](https://admin.migadu.com) for `zaas.at`:

| Address | Purpose |
|---|---|
| `noreply@zaas.at` | Transactional email sender (verification, re-issue links) |
| `dmarc-reports@zaas.at` | Receives DMARC aggregate reports (low volume, internal) |
| `contact@zaas.at` | General contact - displayed on `/imprint` |
| `privacy@zaas.at` | GDPR/DSGVO data subject requests - displayed on `/privacy` and `/datenschutz` |

The following mailboxes are not yet needed; create them when billing is introduced:

| Address | Purpose |
|---|---|
| `support@zaas.at` | User support once registered clients exist |
| `billing@zaas.at` | Stripe billing correspondence |

---

## 5. Email: DNS Records Setup

The records are version-controlled in `infra/tofu/terraform.tfvars`; skip this step if the file already contains `extra_dns_records`.

### 5.1 Add Migadu records to terraform.tfvars

Migadu registration must be completed in order to get the value of the `hosted-email-verify` TXT record.

Edit `infra/tofu/terraform.tfvars` and add:

```hcl
extra_dns_records = [
  # TXT Records (Verification and SPF)
  {
    name   = "@"
    type   = "TXT"
    values = [
        # Verification Record
        "\"hosted-email-verify=replace-me\"",
        # SPF Record
        "\"v=spf1 include:spf.migadu.com -all\""
    ]
  },
  # Mail Exchange (MX) Records
  {
    name   = "@"
    type   = "MX"
    values = ["10 aspmx1.migadu.com.", "20 aspmx2.migadu.com."]
  },
  # DKIM+ARC Key Records
  {
    name   = "key1._domainkey"
    type   = "CNAME"
    values = ["key1.zaas.at._domainkey.migadu.com."]
  },
  {
    name   = "key2._domainkey"
    type   = "CNAME"
    values = ["key2.zaas.at._domainkey.migadu.com."]
  },
  {
    name   = "key3._domainkey"
    type   = "CNAME"
    values = ["key3.zaas.at._domainkey.migadu.com."]
  },
  # DMARC Records (optional)
  {
    name   = "_dmarc"
    type   = "TXT"
    values = ["\"v=DMARC1; p=none; rua=mailto:dmarc-reports@zaas.at\""]
  },
  # Subdomain Addressing (optional)
  {
    name   = "*"
    type   = "MX"
    values = ["10 aspmx1.migadu.com.", "20 aspmx2.migadu.com."]
  },
  # Autoconfig / Autodiscovery Records (optional)
  {
    name   = "autoconfig"
    type   = "CNAME"
    values = ["autoconfig.migadu.com."]
  },
  {
    name   = "_autodiscover._tcp"
    type   = "SRV"
    values = ["0 1 443 autodiscover.migadu.com."]
  },
  {
    name   = "_submissions._tcp"
    type   = "SRV"
    values = ["0 1 465 smtp.migadu.com."]
  },
  {
    name   = "_imaps._tcp"
    type   = "SRV"
    values = ["0 1 993 imap.migadu.com."]
  },
  {
    name   = "_pop3s._tcp"
    type   = "SRV"
    values = ["0 1 995 pop.migadu.com."]
  },
]
```

### 5.2 Preview and apply

```bash
export TF_VAR_hcloud_token="your_token"   # prefix with space to keep out of shell history
make infra-plan   # should show 8 new hcloud_zone_rrset.extra resources
make infra-apply
```

Wait for DNS propagation (typically minutes for Hetzner DNS). Verify:

```bash
dig MX zaas.at
# -> 10 mail.migadu.com.
```

---

## 6. Email: Production .env Configuration

On the server, edit `/opt/zaas/.env` and add/update:

```dotenv
ZAAS_SMTP_USER=noreply@zaas.at
ZAAS_SMTP_PASSWORD=<migadu-noreply-password>
PUBLIC_IMPRINT_EMAIL=contact@zaas.at
PUBLIC_PRIVACY_EMAIL=privacy@zaas.at
```

Restart the API to pick up the new config:

```bash
sudo -u deploy sh -c '
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart api
'
```

Also update **Actions Variables** in GitHub (see [section 3.2](#32-actions-variables)) with the new email values so the next web build picks them up.

---

## 7. Email: End-to-End Verification

Send a test verification email via the API (`/auth/register` requires the
`X-Admin-Token` header - see [Issuing an API Key on Request](#issuing-an-api-key-on-request)):

```bash
curl -X POST https://zaas.at/api/v1/auth/register \
  -H "X-Admin-Token: $ZAAS_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "your-test@example.com", "display_name": "Test"}'
# -> 202 Accepted
```

Check `your-test@example.com` for the verification email. Confirm:
- Email arrives (not in spam)
- Sender shows `ZaaS <noreply@zaas.at>`
- Verification link is valid

**Email setup is complete once this test passes.**

---

## 8. PostgreSQL: Production .env Configuration

`POSTGRES_PASSWORD` is required. The compose file has no default; starting the
stack without it fails immediately with an error message.

Generate a strong password:

```bash
openssl rand -base64 24
```

On the server, edit `/opt/zaas/.env` and add/update:

```dotenv
ZAAS_DB_URL=postgres://zaas:<strong-password>@postgres:5432/zaas?sslmode=disable
POSTGRES_PASSWORD=<strong-password>
```

Restart the stack to pick up the new config:

```bash
sudo -u deploy sh -c '
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d
'
```

Verify PostgreSQL is healthy:

```bash
docker exec deploy-postgres-1 pg_isready -U zaas
# -> deploy-postgres-1:5432 - accepting connections
```

---

## 9. Backups: Timers and Metrics

Two daily backups run on the host, each with its own systemd timer:

| Timer | Runs | Covers |
| ----- | ---- | ------ |
| `zaas-backup.timer` | 03:00 | `pg_dump` of the `zaas` database -> `/var/backups/zaas/{daily,weekly,monthly}/` |
| `zaas-backup-volumes.timer` | 03:30 | tar archives of `caddy_data`, `grafana_data`, `alertmanager_data` -> `/var/backups/zaas/volumes/{daily,weekly,monthly}/` |

The volume run stops Grafana for a few seconds, because `grafana.db` is live SQLite and a hot tar can capture a torn write. `caddy_data` and `alertmanager_data` are written atomically and are tarred while running. Grafana's `./plugins` directory is excluded (~85 MB of re-downloadable plugin code), which keeps the whole volume set under a megabyte per day.

Both use Grandfather-Father-Son rotation (7 daily, 4 weekly, 6 monthly) and both report
their outcome as a Prometheus metric through the node_exporter textfile collector, which
drives the `ZaasBackupFailed` / `ZaasBackupStale` / `ZaasBackupMetricsMissing` alerts.

`postgres_data` is deliberately not in the volume backup: a file-level copy of a running
data directory is not crash-consistent, so the logical `pg_dump` is the right tool for it.
`redis_data` (ephemeral rate-limit counters) and the telemetry volumes (`prometheus_data`,
`loki_data`, `tempo_data`) are excluded as well.

### 9.1 Create the backup and metrics directories

```bash
# 0750, not the mkdir default of 0755: the dumps contain hashed API keys, and the
# offsite reader account (section 12) gets in through group-read on this directory.
sudo install -d -o deploy -g deploy -m 0750 /var/backups/zaas

# Textfile collector drop directory. Owned by deploy because the backup units write
# here; world-readable because node_exporter reads it as a different user and these
# files hold timestamps and byte counts, nothing sensitive.
sudo install -d -o deploy -g deploy -m 0755 /var/lib/node_exporter/textfile_collector
```

### 9.2 Pre-pull the tar helper image

The volume backup tars each volume through a throwaway container. Pulling it once now
means the 03:30 run does not depend on the network:

```bash
sudo -u deploy docker pull debian:13-slim@sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd
```

Keep this digest in sync with `TAR_IMAGE` in `/opt/zaas/deploy/scripts/backup-volumes.sh`.

### 9.3 Install and enable the systemd timers

```bash
sudo cp /opt/zaas/deploy/scripts/zaas-backup.service /etc/systemd/system/
sudo cp /opt/zaas/deploy/scripts/zaas-backup.timer /etc/systemd/system/
sudo cp /opt/zaas/deploy/scripts/zaas-backup-volumes.service /etc/systemd/system/
sudo cp /opt/zaas/deploy/scripts/zaas-backup-volumes.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zaas-backup.timer zaas-backup-volumes.timer
```

> The scripts themselves arrive with `git pull` (the sparse checkout of `deploy/` into
> `/opt/zaas`), so editing a script needs no copy step. The **unit files** do: a changed
> `.service` or `.timer` has to be re-copied and `daemon-reload`ed by hand.

### 9.4 Verify

```bash
systemctl list-timers 'zaas-backup*'
# -> both timers listed with a NEXT elapse

# Test a manual run of each. The volume backup stops Grafana for a few seconds.
sudo systemctl start zaas-backup.service
sudo systemctl start zaas-backup-volumes.service

journalctl -u zaas-backup.service -u zaas-backup-volumes.service --no-pager -n 40

ls -la /var/backups/zaas/daily/          # -> zaas-<date>.sql.gz, mode 0640
ls -la /var/backups/zaas/volumes/daily/  # -> three deploy_*-<date>.tar.gz, mode 0640
```

Confirm the metrics were written and that no `.tmp` files were left behind:

```bash
cat /var/lib/node_exporter/textfile_collector/zaas-backup-*.prom
# -> zaas_backup_last_run_success{backup="postgres"} 1
#    zaas_backup_last_run_success{backup="volumes"} 1

ls /var/backups/zaas/daily/*.tmp /var/backups/zaas/volumes/daily/*.tmp 2>/dev/null
# -> no such file (expected)
```

The metrics only reach Prometheus once the textfile collector is enabled in section 11.

---

## 10. Redis: Production .env Configuration

On the server, edit `/opt/zaas/.env` and add/update:

```dotenv
ZAAS_RATE_LIMIT_BACKEND=redis
ZAAS_REDIS_URL=redis://redis:6379
```

Restart the stack to pick up the new config:

```bash
sudo -u deploy sh -c '
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d
'
```

Verify Redis is healthy:

```bash
docker exec deploy-redis-1 redis-cli ping
# -> PONG
```

Verify the rate limiter is using Redis (check API logs):

```bash
docker logs deploy-api-1 2>&1 | grep "rate limiter"
# -> ... redis rate limiter initialized url=redis://redis:6379 rpm=60
```

Verify Redis metrics are being scraped (from Prometheus):

```
# On the server, or via the SSH tunnel in "Accessing Internal Service UIs":
http://localhost:9090/targets
# -> redis job should show "UP"
```

---

## 11. Node Exporter: Host Metrics

Node Exporter exposes host-level metrics (CPU, memory, disk, network, filesystem) to Prometheus. It runs as an unprivileged systemd service on the host and is scraped by the Prometheus container via `host.docker.internal:9100`.

### 11.1 Download and install

```bash
# Download latest stable release
NODE_EXPORTER_VERSION="1.9.1"
curl -L "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz" \
  -o /tmp/node_exporter.tar.gz

tar -xzf /tmp/node_exporter.tar.gz -C /tmp/
sudo install -m 755 /tmp/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter /usr/local/bin/node_exporter
rm -rf /tmp/node_exporter*
```

### 11.2 Create dedicated system user

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin node_exporter
```

### 11.3 Create systemd service unit

```bash
sudo tee /etc/systemd/system/node_exporter.service > /dev/null << 'EOF'
[Unit]
Description=Prometheus Node Exporter
Documentation=https://github.com/prometheus/node_exporter
After=network.target

[Service]
User=node_exporter
Group=node_exporter
Type=simple
ExecStart=/usr/local/bin/node_exporter \
  --collector.disable-defaults \
  --collector.cpu \
  --collector.meminfo \
  --collector.filesystem \
  --collector.netdev \
  --collector.loadavg \
  --collector.uname \
  --collector.time \
  --collector.stat \
  --collector.textfile \
  --collector.textfile.directory=/var/lib/node_exporter/textfile_collector \
  --web.listen-address=:9100
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
```

`sudo tee` rather than `sudo cat >`: the redirection is performed by your own shell before
`sudo` runs, so `sudo cat > /etc/...` fails with permission denied as a non-root user. Copy
the whole block, including the `EOF`, rather than assembling `ExecStart` by hand - a dropped
flag here is silent, and the one it costs you is usually `--collector.textfile`.

Confirm the flags that matter survived the write, before enabling the service:

```bash
grep -c '^  --collector.textfile' /etc/systemd/system/node_exporter.service
# -> 2   (the collector and its directory - see the note below on why one without
#         the other silently does nothing)
```

> **Why `--collector.disable-defaults` with explicit collectors?** This avoids hundreds of low-value metrics (systemd units, NFS, hardware sensors, etc.) and keeps Prometheus cardinality low. The selected collectors cover all the metrics used by the Grafana dashboard.
>
> `--collector.textfile` is on the list because the backup timers (section 9) report their outcome by dropping `.prom` files into the directory below - that is what feeds the `ZaasBackupStale` alert. Because defaults are disabled, **both** flags are required: `--collector.textfile.directory` on its own does not enable the collector.

### 11.4 Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now node_exporter
```

### 11.5 Allow the Docker bridge network through the firewall

Node Exporter binds to all interfaces by default. UFW is enabled by cloud-init, which only opens ports 22, 80, and 443 - port 9100 is blocked by default, including for the Prometheus container.

`host.docker.internal` (configured via `extra_hosts: host-gateway` in `deploy/docker-compose.yaml`) resolves to the host's own bridge-gateway IP, so a scrape from the `prometheus` container is host-destined traffic that hits UFW's INPUT chain like any other incoming connection. This is not exempt the way container-to-container or published-port traffic is via the FORWARD chain. Without an explicit allow rule, UFW silently drops the connection instead of refusing it, which shows up as a hang rather than an immediate error.

Allow port 9100 only from the Docker bridge subnet used by the compose stack, not the whole internet:

```bash
docker network inspect deploy_default | grep Subnet
# e.g. "Subnet": "172.20.0.0/16"

sudo ufw allow from <bridge-subnet> to any port 9100 proto tcp comment 'node_exporter for prometheus container'
```

This rule is required permanently, not just for the verification step below - Prometheus scrapes this endpoint on every scrape interval (`deploy/prometheus.yaml`).

### 11.6 Verify

```bash
systemctl status node_exporter
# -> Active: active (running)

# Confirm metrics are exposed:
curl -s http://localhost:9100/metrics | grep "^node_cpu_seconds_total" | head -3
# -> node_cpu_seconds_total{cpu="0",mode="idle"} ...

# Confirm Prometheus can reach it (from inside the Prometheus container):
docker exec deploy-prometheus-1 wget -qO- http://host.docker.internal:9100/metrics | head -5
# -> # HELP node_cpu_seconds_total ...

# Check Prometheus targets page:
# http://localhost:9090/targets  ->  node-exporter job should show "UP"
# (on the server, or through the SSH tunnel - see "Accessing Internal Service UIs")
```

Confirm the textfile collector picked up the backup metrics (requires section 9):

```bash
curl -s http://localhost:9100/metrics | grep -E '^(zaas_backup|node_textfile_scrape_error)'
# -> node_textfile_scrape_error 0
#    zaas_backup_last_success_timestamp_seconds{backup="postgres"} ...
```

`node_textfile_scrape_error 1` means a `.prom` file failed to parse, in which case
node_exporter drops **all** textfile metrics, not just the bad one.

---

## 12. Offsite Backups: Read-Only Reader Account

Backups live on the same disk as the data they protect, so a lost server loses both. This
section sets up a dedicated account that a second machine you control uses to **pull**
`/var/backups/zaas` over SSH. The direction matters: the ZaaS server holds no outbound
credentials and cannot reach, modify or delete the offsite copies.

```
backup-server                     zaas-server
     |  ssh -i ~/.ssh/zaas_backup ----->|  authorized_keys:
     |                                  |   restrict,command="/usr/bin/rrsync -ro /var/backups/zaas"
     |<---------- files (read-only) ----|
```

### 12.1 Generate a key pair on the backup server

```bash
ssh-keygen -t ed25519 -f ~/.ssh/zaas_backup -C "zaas offsite backup reader"
```

### 12.2 Create the reader account on the ZaaS server

```bash
# Dedicated, non-sudo account. The shell must be a real one: sshd runs forced
# commands through the login shell, so /usr/sbin/nologin would break the forced
# command below.
sudo useradd -r -m -s /bin/bash zaasbackup

# Group membership is what grants read access to /var/backups/zaas (mode 0750).
sudo usermod -aG deploy zaasbackup
```

### 12.3 Allow the new user through the SSH hardening config

Cloud-init writes `/etc/ssh/sshd_config.d/ssh-hardening.conf` containing
`AllowUsers deploy`. Until the new user is listed there too, sshd rejects it **before**
authentication is attempted, reporting a misleading `Permission denied (publickey)`.

Add a separate drop-in rather than editing the cloud-init-managed file, which would be
rewritten on a server rebuild:

```bash
sudo tee /etc/ssh/sshd_config.d/zz-backup-reader.conf > /dev/null << 'EOF'
AllowUsers deploy zaasbackup
EOF
```

`AllowUsers` is first-match-wins across the merged config and drop-ins are read in lexical
order, hence the `zz-` prefix so this file is parsed after `ssh-hardening.conf`.

Verify before restarting, and **keep an existing SSH session open** while you do:

```bash
sudo sshd -T | grep allowusers
# -> allowusers deploy zaasbackup

sudo systemctl restart ssh
```

### 12.4 Install the key with a forced command

Paste the **public** key from step 12.1 into the reader's `authorized_keys`, prefixed with
the restrictions:

```bash
sudo -u zaasbackup mkdir -p /home/zaasbackup/.ssh
sudo -u zaasbackup chmod 700 /home/zaasbackup/.ssh
sudo -u zaasbackup tee /home/zaasbackup/.ssh/authorized_keys > /dev/null << 'EOF'
restrict,command="/usr/bin/rrsync -ro /var/backups/zaas" ssh-ed25519 AAAA... zaas offsite backup reader
EOF
sudo -u zaasbackup chmod 600 /home/zaasbackup/.ssh/authorized_keys
```

`restrict` disables port/agent/X11 forwarding, pty allocation and user-rc. `rrsync -ro`
locks the session to that one directory and refuses any write, delete or rename.

Confirm `rrsync` is where the forced command expects it:

```bash
ls -l /usr/bin/rrsync
# rsync >= 3.2.4 ships it here; older layouts put it in /usr/share/doc/rsync/scripts/
```

No firewall change is needed - `ufw limit <ssh_port>` already permits SSH.

### 12.5 Verify the key really is confined

From the backup server. All three must **fail**:

```bash
# No interactive shell
ssh -p 2222 -i ~/.ssh/zaas_backup zaasbackup@<server-ip>

# Forced command ignores whatever is requested
ssh -p 2222 -i ~/.ssh/zaas_backup zaasbackup@<server-ip> "cat /etc/passwd"

# Writes are refused
rsync -av -e "ssh -p 2222 -i ~/.ssh/zaas_backup" ./anyfile zaasbackup@<server-ip>:/
```

And this must **succeed**:

```bash
rsync -n -av -e "ssh -p 2222 -i ~/.ssh/zaas_backup" zaasbackup@<server-ip>:/ /tmp/probe/
# -> lists daily/, weekly/, monthly/, volumes/
```

Note the source path is `:/`. With `rrsync` the remote path is relative to the locked root,
so `/` here means `/var/backups/zaas`; an absolute host path fails.

Then set up the pull itself - see [Offsite Backups: Manual Pull](#offsite-backups-manual-pull).

---

## 13. Slack Alert Notifications

Without this section every Prometheus alert - including `ZaasApiDown` and `ZaasBackupStale` -
is visible only to someone who happens to open the Alertmanager UI. This delivers them to a
Slack channel through an Incoming Webhook.

> **Prerequisite: port 9093 must not be reachable from the internet before you do this.**
> The Alertmanager API accepts unauthenticated writes, so anyone who can reach it can `POST`
> an arbitrary alert that Alertmanager will then deliver to Slack looking exactly like a
> genuine one - an anonymous phishing channel into your team's chat. Confirm the lockdown
> from [section 1.4](#14-firewall) is in place first:
>
> ```bash
> curl -sS --connect-timeout 5 http://<server-ip>:9093/api/v2/status
> # must fail or time out
> ```

Incoming Webhooks work on the Slack **Free** plan. Two Free-plan limits are worth knowing:
the workspace is capped at 10 apps/integrations (this app counts as one), and message
history is 90 days - so Slack is the notification channel, not the alert archive. Prometheus
`/alerts` and the Alertmanager UI remain the record.

### 13.1 Create the Slack app and webhook

1. Go to <https://api.slack.com/apps> and choose **Create New App -> From scratch**.
2. Name it (e.g. `ZaaS Alerts`) and pick the workspace.
3. Open **Incoming Webhooks** and toggle **Activate Incoming Webhooks** on.
4. Choose **Add New Webhook to Workspace**, select the target channel (e.g. `#zaas-alerts`),
   and authorize.
5. Copy the generated URL. It has the shape
   `https://hooks.slack.com/services/<workspace-id>/<channel-id>/<token>`.

> **Treat this URL as a password.** Anyone holding it can post to that channel. It is not
> tied to a user account and cannot be scoped further; the only remediation for a leak is to
> revoke the webhook in the Slack app config and issue a new one.

### 13.2 Write the secret onto the server

Alertmanager does not expand environment variables in its config, so the URL cannot live in
`.env`. It is read from a file instead:

The Alertmanager container runs as uid **65534** (`nobody`), so the directory and file must be
owned by that uid. Creating them as `root:root 0700` looks tighter but leaves Alertmanager
unable to open its own secret - and, true to form, it fails silently at notify time rather
than at startup.

```bash
# Owned by the container's uid, not root - 65534 cannot traverse a root-owned 0700 directory.
sudo install -d -o 65534 -g 65534 -m 0700 /opt/zaas/deploy/secrets

# printf, not echo: a trailing newline becomes part of the URL.
printf '%s' 'https://hooks.slack.com/services/<workspace-id>/<channel-id>/<token>' \
  | sudo tee /opt/zaas/deploy/secrets/slack_api_url > /dev/null

sudo chown 65534:65534 /opt/zaas/deploy/secrets/slack_api_url
sudo chmod 0600 /opt/zaas/deploy/secrets/slack_api_url
```

Only uid 65534 and root can read it. On the host it shows as owned by `nobody`.

Confirm the container can actually read it - this is the step that catches the permission
mistake, and it needs no restart, since the file is read at notify time:

```bash
docker exec deploy-alertmanager-1 cat /etc/alertmanager/secrets/slack_api_url
# -> the webhook URL, with no trailing newline
```

`deploy/secrets/` is gitignored, so the secret never enters the repository, and
`git pull --ff-only` in the redeploy flow does not touch it.

### 13.3 Recreate Alertmanager

The compose file gained a new bind mount for that directory. A new mount **is** a config
diff, so unlike a change to a mounted file's contents this does recreate the container:

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml \
  --env-file /opt/zaas/.env up -d --no-deps alertmanager

docker exec deploy-alertmanager-1 ls -l /etc/alertmanager/secrets
# -> slack_api_url
```

Prometheus also needs to reload to pick up the `ZaasWatchdog` rule. `--web.enable-lifecycle`
is not set, so restart it rather than POSTing to `/-/reload`:

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml \
  --env-file /opt/zaas/.env restart prometheus
```

### 13.4 Verify delivery

**Do not skip this.** A missing or wrong secret file does not stop Alertmanager starting and
logs nothing at all - delivery simply fails at send time. Test-fire an alert rather than
waiting for a real one:

```bash
docker exec deploy-alertmanager-1 amtool alert add ZaasSlackTest \
  severity=warning \
  --annotation='description="Delivery test - safe to ignore."' \
  --alertmanager.url=http://localhost:9093
```

The annotation value is double-quoted **inside** the argument. `amtool` parses these with the
UTF-8 matchers parser, which needs quoting for any value containing spaces or punctuation;
without it you get a `level=WARN ... incompatible` line and a fallback to the classic parser.
The alert is still submitted either way, and the annotation is identical - the quoting just
keeps the output clean.

A yellow `[FIRING] ZaasSlackTest` message should reach the channel within ~30 seconds
(`group_wait`). It clears itself after five minutes, and the `[RESOLVED]` message follows one
`group_interval` (5m) later.

Within roughly a minute of the Prometheus restart, a green **ZaaS alerting heartbeat**
message should also arrive - that is `ZaasWatchdog`, which repeats once every 24h. Its whole
purpose is that its *absence* tells you delivery has broken. See
[ZaasWatchdog](#zaaswatchdog).

### 13.5 Rotating the webhook

Revoke the old webhook in the Slack app config, add a new one, then overwrite the file as in
13.2. The file is read at notification time rather than at startup, so no restart should be
needed - confirm with the 13.4 test-fire rather than assuming it.

---

# Part 2: Operational Procedures

## Restarting the Server

**When:** Applying kernel updates, recovering from a hung host, or any other situation requiring a full reboot.

Reboot from inside the OS so services get a chance to shut down cleanly, rather than using a Hetzner Cloud Console power cycle:

```bash
ssh -p 2222 <user_name>@<server-ip>
sudo reboot
```

No manual startup steps are needed afterward: every service in `deploy/docker-compose.yaml` runs with `restart: unless-stopped`, so Docker brings the full stack back up once the daemon starts. `node_exporter` and `zaas-backup.timer` are enabled systemd services (see sections 9 and 11) and start automatically as well.

**Verify everything came back up:**

```bash
# Wait for SSH to come back, then check container status
ssh -p 2222 <user_name>@<server-ip>
docker ps --format 'table {{.Names}}\t{{.Status}}'
# -> all containers should show "Up"

# Check the host-level systemd services
systemctl status node_exporter zaas-backup.timer
# -> both should show "active"

# Check the API responds
curl https://zaas.at/healthz
# -> {"status":"ok"}
```

If any container isn't `Up`, check its logs (`docker logs <container-name> --since 5m`) - see the [First Response Checklist](#first-response-checklist) and the relevant alert playbook below.

---

## Releases

The release workflow is triggered by pushing a version tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The workflow runs `git-cliff` to generate release notes, builds multi-arch Docker images, and creates a GitHub Release. Pre-release tags (e.g. `v1.0.0-rc.1`) are automatically marked as pre-releases.

---

## DMARC Policy Tightening

**When:** A few weeks after email deployment, once DMARC aggregate reports confirm clean alignment.

Check `dmarc-reports@zaas.at` for incoming DMARC aggregate reports. Once reports show consistent SPF/DKIM alignment with no failures:

**Step 1: Tighten to quarantine** (failed mails go to spam)

In `infra/tofu/terraform.tfvars`, change the DMARC record:

```hcl
# from:
value = "\"v=DMARC1; p=none; rua=mailto:dmarc-reports@zaas.at\""
# to:
value = "\"v=DMARC1; p=quarantine; rua=mailto:dmarc-reports@zaas.at\""
```

Apply: `make infra-apply`

Monitor for another week. If still clean:

**Step 2: Tighten to reject** (failed mails are dropped)

```hcl
value = "\"v=DMARC1; p=reject; rua=mailto:dmarc-reports@zaas.at\""
```

Apply: `make infra-apply`

---

## Issuing an API Key on Request

**Why:** `POST /auth/register` and `POST /auth/reissue` require the `X-Admin-Token`
header - see `docs/explanation/design-decisions.md` for the reasoning.
`POST /auth/verify` stays public.

**Procedure:** a prospective user emails `contact@zaas.at` requesting a key (new
registration) or a reissue (lost/compromised key). Run the corresponding request
using the `ZAAS_ADMIN_TOKEN` value from the server's `.env`:

```bash
# New registration
curl -X POST https://zaas.at/api/v1/auth/register \
  -H "X-Admin-Token: $ZAAS_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "display_name": "Requester Name"}'

# Reissue (revokes the old key on verification)
curl -X POST https://zaas.at/api/v1/auth/reissue \
  -H "X-Admin-Token: $ZAAS_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

Both endpoints always return `202` regardless of outcome (prevents email
enumeration), so this is not a reliable success signal - confirm success by asking
the requester to check for the verification email and complete it via the `/verify`
link.

---

## Accessing the API Container Directly

**Why:** The `api` service publishes no port to the host (unlike `caddy`, which
publishes 80/443) - it's only reachable from other containers on the compose
network. `curl http://localhost:8080/...` on the host will not work. The `api`
image is also `FROM scratch` (no shell, no `wget`, no `curl`), so you cannot
`docker exec` into it either. Reach it by execing into `caddy` instead, which
is on the same network and has `curl` available:

```bash
# Call an endpoint directly, bypassing the domain and Caddy's routing rules
docker exec deploy-caddy-1 curl -s http://api:8080/api/v1/dice

# Open a shell to poke around further (sh, not bash - this is Alpine/BusyBox)
docker exec -it deploy-caddy-1 sh
```

Useful when you want to rule out Caddy/DNS/TLS as the cause of a problem and
confirm whether the API itself is behaving correctly.

---

## PostgreSQL: Manual Database Access

Connect to PostgreSQL:

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas
```

See [Client Administration](#client-administration-sql-reference) below for the full SQL reference.

---

## Client Administration (SQL Reference)

Admin REST endpoints and a CLI are planned for a future release; until then, use direct PostgreSQL access.

**Connect to PostgreSQL:**

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas
```

**List all clients:**

```sql
SELECT id, email, display_name, api_key_prefix, rate_limit_rpm,
       created_at, verified_at, revoked_at
FROM clients
ORDER BY created_at DESC;
```

**List unverified clients (pending email verification):**

```sql
SELECT email, display_name, created_at
FROM clients
WHERE verified_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC;
```

**Look up a specific client by email or key prefix:**

```sql
SELECT * FROM clients WHERE email = 'user@example.com';
SELECT * FROM clients WHERE api_key_prefix = 'zaas_a1b2';
```

**View a client's current rate limit usage (Redis - run from Redis CLI):**

```bash
docker exec -it deploy-redis-1 redis-cli
> KEYS rate:client:*
> ZCARD rate:client:<client-id>
```

**Revoke a client's API key:**

```sql
UPDATE clients
SET revoked_at = now()
WHERE email = 'user@example.com';
```

**Reinstate a revoked client (undo revocation):**

```sql
UPDATE clients
SET revoked_at = NULL
WHERE email = 'user@example.com';
```

**Update a client's rate limit:**

```sql
UPDATE clients
SET rate_limit_rpm = 1200
WHERE email = 'user@example.com';
```

**List active (unexpired, unused) verification tokens:**

```sql
SELECT email, type, expires_at, created_at
FROM verification_tokens
WHERE used_at IS NULL AND expires_at > now()
ORDER BY expires_at;
```

**Clean up expired/used tokens manually:**

```sql
DELETE FROM verification_tokens
WHERE used_at IS NOT NULL OR expires_at < now() - interval '7 days';
```

**Count clients by status:**

```sql
SELECT
  COUNT(*) FILTER (WHERE verified_at IS NOT NULL AND revoked_at IS NULL) AS active,
  COUNT(*) FILTER (WHERE verified_at IS NULL) AS pending_verification,
  COUNT(*) FILTER (WHERE revoked_at IS NOT NULL) AS revoked
FROM clients;
```

---

## Offsite Backups: Manual Pull

Run **on the backup server**, not on the ZaaS server. Requires the reader account from
section 12.

The script is `deploy/scripts/pull-backups.sh` from this repository. It is checked in here
for version control; copy it to the backup server (or clone the repo there) and run it from
that machine.

### One-time setup on the backup server

Pin the ZaaS server's host key so a silent host-key swap cannot go unnoticed - an
occasional manual command is exactly the case where it would:

```bash
ssh-keyscan -p 2222 <server-ip> >> ~/.ssh/known_hosts
```

### Pulling

```bash
export ZAAS_BACKUP_HOST=<server-ip>
./pull-backups.sh
```

Defaults, all overridable by environment variable:

| Variable | Default |
| -------- | ------- |
| `ZAAS_BACKUP_USER` | `zaasbackup` |
| `ZAAS_BACKUP_PORT` | `2222` |
| `ZAAS_BACKUP_KEY` | `~/.ssh/zaas_backup` |
| `ZAAS_BACKUP_DEST` | `/var/backups/zaas-offsite` |

The pull deliberately does **not** pass `--delete`: source-side GFS rotation would
otherwise propagate here and delete exactly the history this copy exists to preserve. The
offsite copy therefore grows by a few MB plus one SQL dump per pull - prune it by hand if
it ever matters.

### Verify a pulled archive matches the source

```bash
# On the backup server
sha256sum /var/backups/zaas-offsite/daily/zaas-<date>.sql.gz

# On the ZaaS server
sha256sum /var/backups/zaas/daily/zaas-<date>.sql.gz
```

---

## Accessing Internal Service UIs

Prometheus, Alertmanager, Loki, Tempo and Grafana bind to `127.0.0.1` on the server and are
blocked at the Hetzner Cloud Firewall. They are not reachable from the internet, by design.

Only two things are public: Caddy on 80/443, and Grafana through it at
`https://grafana.<domain>`. Everything else needs either an SSH session or a tunnel.

### From the server

Nothing special - `localhost` works exactly as it always did:

```bash
curl -s localhost:9093/api/v2/alerts
curl -s localhost:9090/api/v1/rules
curl -s 'localhost:3100/loki/api/v1/labels'
```

Every diagnostic command elsewhere in this runbook assumes you are on the server.

### From a workstation

Forward the ports you need over SSH, then use `localhost` on your own machine:

```bash
ssh -p 2222 -N \
  -L 9090:127.0.0.1:9090 \
  -L 9093:127.0.0.1:9093 \
  -L 3100:127.0.0.1:3100 \
  -L 3200:127.0.0.1:3200 \
  <user_name>@<server-ip>
```

Leave that running and open `http://localhost:9090/targets`, `http://localhost:9093`, and so
on in a browser. `-N` means "no remote command", so it just holds the tunnel open.

Grafana needs no tunnel - use `https://grafana.<domain>`. The loopback binding on 3000 is a
break-glass path for when Caddy itself is the problem; add `-L 3000:127.0.0.1:3000` then.

### Why it is set up this way

Docker publishes container ports by DNAT'ing them into the `FORWARD` chain, which never
traverses `INPUT` where ufw's rules live. A port published on `0.0.0.0` is therefore reachable
from the internet no matter what `ufw status` says. Two independent measures close this:

1. `deploy/docker-compose.yaml` binds each of these services to `127.0.0.1`, so Docker never
   opens them beyond the host.
2. The Hetzner Cloud Firewall (`infra/tofu/modules/hcloud_server/main.tf`) drops everything
   except SSH, HTTP, HTTPS and ICMP at Hetzner's edge, before packets reach the host - which
   is what makes a future accidental `0.0.0.0` binding harmless.

Adding a genuinely public port therefore needs **both** a compose change and a firewall rule.
See [gotchas.md](gotchas.md) for the full explanation of the ufw/Docker interaction.

---

# Part 3: Alert Playbooks

This part documents the response procedure for each Prometheus alert defined in `deploy/prometheus.rules.yaml`. Alert notifications are delivered via Alertmanager (configured in `deploy/alertmanager.yaml`). The Alertmanager UI is at `http://localhost:9093` from the server, or through the SSH tunnel in [Accessing Internal Service UIs](#accessing-internal-service-uis). It is not exposed to the internet.

## First Response Checklist

When paged, run through these steps before diving into a specific alert playbook:

1. Open Grafana ZaaS Overview: `https://grafana.<domain>`. Check for red panels.
2. Check API health endpoint: `curl https://zaas.at/healthz` - should return `{"status":"ok"}`.
3. Check container status on the server:
   ```bash
   ssh -p 2222 <user>@<server-ip>
   docker ps --format 'table {{.Names}}\t{{.Status}}'
   ```
4. Check Caddy logs in Loki (Grafana -> Explore -> Loki, filter `service_name="caddy"`).
5. Check API logs: `docker logs deploy-api-1 --since 15m 2>&1 | tail -50`
6. Check PostgreSQL exporter target: Grafana -> Explore -> Prometheus, query `up{job="postgres"}`.
7. Check Redis exporter target: query `up{job="redis"}`.
8. If the Grafana dashboard shows no data at all, check the OTel Collector (see [ZaasCollectorDown](#zaascollectordroppeddata)).
9. After diagnosing, update the Alertmanager silence if work is underway to suppress repeated pages.

---

## ZaasApiDown

**Severity:** critical
**Condition:** `up{job="zaas-api"} == 0` for 2 minutes.

**Symptom:** Prometheus cannot scrape the `zaas-api` metrics endpoint. All API traffic is failing.

**Likely causes:**
- API container crashed or was stopped.
- Out-of-memory kill (OOM).
- Database unavailable - API refused to start or panicked.
- Deployment in progress.

**Diagnostic steps:**

```bash
# Check container status
docker ps -a --filter name=deploy-api

# Check recent logs
docker logs deploy-api-1 --since 10m 2>&1 | tail -100

# Check OOM kills in last hour
dmesg -T | grep -i "killed process" | tail -20

# Attempt a health check directly (the api image has no shell/wget of its own -
# see "Accessing the API Container Directly" below)
docker exec deploy-caddy-1 curl -s http://api:8080/healthz
```

**Remediation:**

```bash
# Restart the API container
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart api

# If the database is down, fix Postgres first (see ZaasPostgresDown), then restart
# If OOM: check memory usage, consider reducing GOMAXPROCS or adding swap
```

---

## ZaasHighErrorRate

**Severity:** critical
**Condition:** 5xx responses exceed 5% of total traffic over 5 minutes.

**Symptom:** A significant portion of API requests are returning 500-level errors.

**Likely causes:**
- Database connection failures or pool exhaustion.
- Redis unavailable (affects rate limiting - fail-open may mask or cause cascading errors).
- Unhandled panic in a handler.
- SMTP unavailable (only affects `/auth/*` endpoints).
- Bad deployment - newly deployed code has a bug.

**Diagnostic steps:**

```bash
# Find error log lines in the last 15 minutes
docker logs deploy-api-1 --since 15m 2>&1 | grep '"level":"ERROR"'

# Check for panic stack traces
docker logs deploy-api-1 --since 15m 2>&1 | grep -A 10 "panic"

# In Grafana: Explore -> Loki -> {service_name="zaas-api"} |= "ERROR"
# Look for the http.route and error fields to isolate the failing endpoint.

# Check database connectivity
docker exec deploy-postgres-1 pg_isready -U zaas

# Check Redis connectivity
docker exec deploy-redis-1 redis-cli ping
```

**Remediation:**

```bash
# If it is a database issue: see ZaasPostgresDown
# If it is a Redis issue: see ZaasRedisDown
# If it is a bad deployment: roll back by deploying the previous image tag
# If cause unknown: restart the API as a temporary measure
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart api
```

---

## ZaasHighP99Latency

**Severity:** warning
**Condition:** p99 response latency exceeds 1 second over 5 minutes.

**Symptom:** Slow responses for the tail of users. API is up but degraded.

**Likely causes:**
- Slow database queries (missing index, lock contention, autovacuum running).
- Redis latency (network or high memory pressure).
- CPU saturation on the host.
- Large response payloads (e.g. `/api/v1/lorem` with a high `paragraphs` value).

**Diagnostic steps:**

```bash
# Check CPU and memory on the host
docker stats --no-stream

# Check slow Postgres queries (requires pg_stat_statements extension)
docker exec -it deploy-postgres-1 psql -U zaas -d zaas -c "
  SELECT query, calls, total_exec_time / calls AS avg_ms, rows
  FROM pg_stat_statements
  ORDER BY avg_ms DESC
  LIMIT 10;
"

# Check Redis slowlog
docker exec deploy-redis-1 redis-cli SLOWLOG GET 10

# In Grafana: ZaaS Overview -> Request Duration p99 panel -> drill into http_route
# to identify which endpoint is slow.
```

**Remediation:**

If a slow query is identified, run `EXPLAIN ANALYZE` and add an index if missing. If CPU is saturated, check for runaway containers with `docker stats`.

---

## ZaasHighRateLimitRate

**Severity:** warning
**Condition:** 429 responses exceed 10 per second over 5 minutes.

**Symptom:** A high volume of requests is being rate-limited. May indicate abuse or a misconfigured client.

**Likely causes:**
- A single IP or API key hammering the API.
- A misconfigured load tester or bot.
- A legitimate client that needs a higher rate limit.
- Global rate limit misconfiguration (ZAAS_RATE_LIMIT_RPM set too low).

**Diagnostic steps:**

```bash
# In Grafana: Loki -> filter by http_response_status_code=429
# Look for the source IP or client_id in the log fields.

# Check the current rate limit configuration (docker inspect reads container
# metadata from the host - no exec into the scratch-based api image needed)
docker inspect deploy-api-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep ZAAS_RATE_LIMIT
```

**Remediation:**

If abuse is confirmed, block the IP at the Caddy or UFW level. If a legitimate client needs a higher rate limit, update their `rate_limit_rpm` in PostgreSQL (see [Client Administration](#client-administration-sql-reference)).

---

## ZaasDiskSpaceLow

**Severity:** warning (< 10%), critical (< 5%)
**Condition:** Available disk on `/` drops below threshold.

**Symptom:** The server is running out of disk space.

**Likely causes:**
- PostgreSQL data growth.
- Docker image and container layer accumulation.
- Log accumulation.
- Prometheus or Loki storage growth (both now have persistent volumes).
- Backup rotation not running, so dumps and volume archives accumulate.

**Diagnostic steps:**

```bash
# Find largest consumers
df -h /
du -sh /var/lib/docker/*
du -sh /var/backups/zaas/ /var/backups/zaas/volumes/

# Check Docker disk usage
docker system df
```

**Remediation:**

```bash
# Remove unused Docker images and containers
docker system prune -f

# Remove old backup files (keep at least 7 days). Both backup sets rotate on their
# own; only step in here if rotation has fallen behind.
ls -lt /var/backups/zaas/daily/ /var/backups/zaas/volumes/daily/
# rm files older than 7 days manually after confirming

# If Prometheus storage is the cause, reduce retention:
# Add --storage.tsdb.retention.time=15d to the prometheus command in docker-compose.yaml
```

---

## ZaasPostgresDown

**Severity:** critical
**Condition:** `up{job="postgres"} == 0` for 2 minutes (postgres-exporter unreachable).

**Symptom:** PostgreSQL is down or unreachable. Auth and all database-backed operations fail.

**Likely causes:**
- Container crashed.
- Disk full - PostgreSQL cannot write WAL.
- Corrupted data directory.

**Diagnostic steps:**

```bash
# Check container status
docker ps -a --filter name=deploy-postgres

# Check PostgreSQL logs
docker logs deploy-postgres-1 --since 10m 2>&1 | tail -50

# Check if pg_isready responds
docker exec deploy-postgres-1 pg_isready -U zaas
```

**Remediation:**

```bash
# Restart PostgreSQL
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart postgres

# If disk is full: free space first (see ZaasDiskSpaceLow), then restart

# If data is corrupted: see PostgreSQL: Restore from pg_dump below
```

---

## ZaasRedisDown

**Severity:** critical
**Condition:** `up{job="redis"} == 0` for 2 minutes (redis-exporter unreachable).

**Symptom:** Redis is down or unreachable. Rate limiting fails open (the API continues to serve but rate limits are not enforced for `/api/v1/*`; `/auth/*` returns 503 in fail-closed mode).

**Likely causes:**
- Container crashed.
- Out-of-memory kill.
- Disk full (Redis persistence enabled).

**Diagnostic steps:**

```bash
# Check container status
docker ps -a --filter name=deploy-redis

# Check Redis logs
docker logs deploy-redis-1 --since 10m 2>&1 | tail -50

# Attempt a direct ping
docker exec deploy-redis-1 redis-cli ping
```

**Remediation:**

```bash
# Restart Redis
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart redis

# Redis data is in the redis_data volume; restart should recover state from the RDB snapshot.
```

---

## ZaasNodeExporterDown

**Severity:** warning
**Condition:** `up{job="node-exporter"} == 0` for 2 minutes.

**Symptom:** Host metrics stop. No user-facing impact - the API keeps serving - but the
Grafana CPU, memory, disk and network panels go empty, and **the disk-space alerts go
blind**: `ZaasDiskSpaceLow` and `ZaasDiskSpaceCritical` evaluate to no data, which never
fires. A filling disk will not alert while this is unresolved, so treat it as time-boxed
rather than deferrable.

`ZaasBackupMetricsMissing` will also fire an hour later, because the backup metrics reach
Prometheus through this exporter's textfile collector. If both alerts are firing, this is the
cause and the backup alert is a symptom.

**Likely causes:**
- The service is stopped, crashed, or was never installed (section 11). It is a host systemd
  service, not a compose service, so a redeploy neither restarts nor notices it.
- The UFW rule for port 9100 is missing, was scoped to the wrong subnet, or the Docker bridge
  subnet changed. UFW *drops* rather than refuses, so the scrape hangs until timeout instead
  of failing fast - see [section 11.5](#115-allow-the-docker-bridge-network-through-the-firewall).
- The server was rebuilt and section 11 was not re-run.

**Diagnostic steps:**

```bash
# Is the service running at all?
systemctl status node_exporter

# Does it answer locally? (rules the process in or out before looking at the firewall)
curl -s --max-time 5 http://localhost:9100/metrics | head -3

# Can the Prometheus container reach it? A hang here rather than an error means UFW.
docker exec deploy-prometheus-1 wget -qO- --timeout=5 http://host.docker.internal:9100/metrics | head -3

# Is the firewall rule still present, and does it still match the bridge subnet?
sudo ufw status numbered | grep 9100
docker network inspect deploy_default | grep Subnet
```

**Remediation:**

```bash
# Service stopped or crashed
sudo systemctl restart node_exporter
journalctl -u node_exporter --no-pager -n 50

# Not installed, or the server was rebuilt: work section 11 end to end
# (install, user, unit, enable, and the UFW rule - all four are required)

# Firewall rule missing or pointing at the wrong subnet
sudo ufw allow from <bridge-subnet> to any port 9100 proto tcp comment 'node_exporter for prometheus container'
```

Confirm the fix on the Prometheus targets page (`http://localhost:9090/targets`, through the
SSH tunnel) rather than by waiting for the alert to resolve.

---

## ZaasCollectorDroppedData

**Severity:** warning (dropped data) / critical (collector down)
**Conditions:**
- `rate(otelcol_exporter_send_failed_spans_total[5m]) > 0` - dropping spans
- `rate(otelcol_exporter_send_failed_metric_points_total[5m]) > 0` - dropping metrics
- `rate(otelcol_exporter_send_failed_log_records_total[5m]) > 0` - dropping logs
- `up{job="otel-collector"} == 0` for 2 minutes - collector down

**Symptom:** Telemetry data is not reaching Tempo/Prometheus/Loki. Grafana shows gaps or no data. Correlation from logs to traces does not work.

**Note:** This is an observability failure, not a user-facing failure. The API continues to serve traffic.

**Likely causes:**
- Tempo, Prometheus, or Loki are down or unreachable.
- Collector OOM (no `memory_limiter` was configured prior to task 2.4; ensure that fix is applied).
- Network issue between Collector container and backends.
- Collector configuration error after a config change.

**Diagnostic steps:**

```bash
# Check Collector container status and logs
docker ps -a --filter name=deploy-otel-collector
docker logs deploy-otel-collector-1 --since 10m 2>&1 | tail -100

# Check backend health
docker ps -a --filter name=deploy-tempo
docker ps -a --filter name=deploy-prometheus
docker ps -a --filter name=deploy-loki

# In Prometheus (if it is still up): check otelcol_* metrics
# http://localhost:9090/graph?g0.expr=otelcol_exporter_send_failed_spans_total
```

**Remediation:**

```bash
# If a backend is down, restart it first, then restart the Collector
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart tempo
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart otel-collector

# If the Collector config was recently changed and it fails to start, revert the change
```

---

## ZaasBackupStale

Covers `ZaasBackupFailed`, `ZaasBackupStale` and `ZaasBackupMetricsMissing`.

**Severity:** warning (run failed, metrics missing), critical (no success in > 26h)
**Condition:** A daily backup did not succeed, or its metric is gone entirely.

**Symptom:** The backup that a restore would depend on is not being produced. Nothing is
broken in production yet - this alert exists so a bad restore is not the first sign.

**Likely causes:**
- node_exporter is down, so the metrics never reach Prometheus. If **both** backups' metrics
  are missing at once, check [ZaasNodeExporterDown](#zaasnodeexporterdown) first - two
  independently scheduled backups rarely fail in the same instant, but one dead exporter
  takes out both.
- The backup script failed: PostgreSQL container down, Docker unavailable, disk full.
- The timer is disabled or was never enabled (`ZaasBackupMetricsMissing`).
- The unit files were changed in git but never re-copied to `/etc/systemd/system/`.
- The node_exporter textfile collector is not enabled, or a `.prom` file failed to parse -
  in which case node_exporter drops **all** textfile metrics, not just the bad one.

**Diagnostic steps:**

```bash
# Did the timers run, and are they still armed?
systemctl list-timers 'zaas-backup*'
systemctl status zaas-backup.service zaas-backup-volumes.service

# Why did the run fail?
journalctl -u zaas-backup.service -u zaas-backup-volumes.service --since '2 days ago'

# What is actually on disk?
ls -lt /var/backups/zaas/daily/ /var/backups/zaas/volumes/daily/
df -h /

# What is the metric saying, and did node_exporter read it?
cat /var/lib/node_exporter/textfile_collector/zaas-backup-*.prom
curl -s http://localhost:9100/metrics | grep -E '^(zaas_backup|node_textfile_)'
```

**Remediation:**

```bash
# Re-run the failed backup by hand and watch it
sudo systemctl start zaas-backup.service
journalctl -u zaas-backup.service --no-pager -n 40

# Timer not armed
sudo systemctl enable --now zaas-backup.timer zaas-backup-volumes.timer

# Unit file changed in git but not installed (git pull does not touch /etc/systemd/system)
sudo cp /opt/zaas/deploy/scripts/zaas-backup*.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload

# node_textfile_scrape_error is 1: find the malformed file, fix or delete it,
# then re-run the backup to regenerate it
ls -l /var/lib/node_exporter/textfile_collector/
```

If the disk is full, work the [ZaasDiskSpaceLow](#zaasdiskspacelow) playbook first - a
full disk fails the backup and the alert clears itself once space is freed and the next run
succeeds.

`ZaasBackupFailed` stays firing until the **next** run succeeds, which by default is the
following day. Re-running the unit by hand clears it sooner.

---

## ZaasWatchdog

**Severity:** none
**Condition:** `vector(1)` - always true, by design.

**Symptom:** None. This alert is *supposed* to be firing at all times, and seeing it in
Prometheus `/alerts` or the Alertmanager UI is normal. Do not silence it and do not try to
make it stop.

**What is actionable is its absence.** It delivers one green "ZaaS alerting heartbeat"
message to Slack every 24 hours. If that message stops arriving, the alerting pipeline is
broken - not the service. Every other alert in this runbook is being lost silently.

**Why it exists:** if `slack_api_url_file` is missing or wrong, Alertmanager starts
completely cleanly, logs nothing, and even passes `amtool check-config`. Delivery fails only
at send time. Without a heartbeat there is no way to tell a broken webhook from a quiet week.

**Diagnostic steps** (work top-down; each rules out one hop):

```bash
# 1. Is Prometheus evaluating the rule?
#    http://localhost:9090/alerts -> ZaasWatchdog should be FIRING

# 2. Did Prometheus hand it to Alertmanager?
#    http://localhost:9093 -> ZaasWatchdog should be listed
docker exec deploy-alertmanager-1 amtool alert \
  --alertmanager.url=http://localhost:9093

# 3. Is the secret actually present and readable in the container?
docker exec deploy-alertmanager-1 ls -l /etc/alertmanager/secrets/slack_api_url

# 4. Did Alertmanager try and fail to notify? This is where a bad URL shows up.
docker logs deploy-alertmanager-1 --since 24h 2>&1 | grep -i "notify\|slack\|error"

# 5. Is the config still what you think it is?
docker exec deploy-alertmanager-1 amtool check-config /etc/alertmanager/alertmanager.yaml
docker exec deploy-alertmanager-1 amtool config routes test \
  --config.file=/etc/alertmanager/alertmanager.yaml severity=none
# -> slack-watchdog
```

**Remediation:**

- Secret file missing after a server rebuild: redo [section 13.2](#13-slack-alert-notifications).
- Trailing newline in the URL (the classic `echo` mistake): rewrite it with `printf`.
- Slack revoked or rotated the webhook: issue a new one and update the file.
- An accidental silence covering `severity="none"`: remove it in the Alertmanager UI.

Confirm the fix with the test-fire in section 13.4 rather than waiting 24h for the next
heartbeat.

---

## PostgreSQL: Restore from pg_dump

Use this procedure when the PostgreSQL data volume is lost or corrupted and a `pg_dump` backup is available.

**Backups are stored at:** `/var/backups/zaas/daily/` on the host (created by the systemd timer in section 9).

### Step 1: Identify the backup to restore

```bash
ls -lt /var/backups/zaas/daily/
# Choose the most recent file, e.g. zaas-2026-05-31.sql.gz
```

### Step 2: Stop the API (to prevent writes during restore)

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env stop api
```

### Step 3: Drop and recreate the database

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d postgres -c "DROP DATABASE IF EXISTS zaas;"
docker exec -it deploy-postgres-1 psql -U zaas -d postgres -c "CREATE DATABASE zaas;"
```

### Step 4: Restore from the backup

```bash
gunzip -c /var/backups/zaas/daily/zaas-2026-05-31.sql.gz \
  | docker exec -i deploy-postgres-1 psql -U zaas -d zaas -v ON_ERROR_STOP=1
```

`-v ON_ERROR_STOP=1` is not optional: without it `psql` continues past failing statements
and exits 0, so a partially restored database reports success.

### Step 5: Verify the restore

```bash
docker exec -it deploy-postgres-1 psql -U zaas -d zaas -c "SELECT COUNT(*) FROM clients;"
# Should return a non-zero count if clients existed before the failure.
```

### Step 6: Restart the API

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env start api
```

Verify: `curl https://zaas.at/healthz`

---

## Docker Volumes: Restore from tar backup

Use this procedure when `caddy_data`, `grafana_data` or `alertmanager_data` is lost or
corrupted. Archives are created by the timer in section 9.

**Backups are stored at:** `/var/backups/zaas/volumes/{daily,weekly,monthly}/` on the host,
named `deploy_<volume>-<date>.tar.gz`.

### Step 1: Identify the archive

```bash
ls -lt /var/backups/zaas/volumes/daily/
# e.g. deploy_grafana_data-2026-08-01.tar.gz
```

### Step 2: Stop the service that owns the volume

| Volume | Service |
| ------ | ------- |
| `deploy_caddy_data` | `caddy` |
| `deploy_grafana_data` | `grafana` |
| `deploy_alertmanager_data` | `alertmanager` |

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml \
  --env-file /opt/zaas/.env stop grafana
```

### Step 3: Wipe and repopulate the volume in place

Restoring into the existing volume keeps it attached to the compose project, so no
container has to be recreated.

```bash
docker run --rm \
  -v deploy_grafana_data:/target \
  -v /var/backups/zaas/volumes/daily:/backup:ro \
  debian:13-slim@sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd \
  sh -c 'rm -rf /target/..?* /target/.[!.]* /target/* && \
         tar xzf /backup/deploy_grafana_data-2026-08-01.tar.gz --numeric-owner -C /target'
```

The helper runs as root so `tar` can restore ownership, and `--numeric-owner` on extract
matters as much as it did on create: Grafana runs as uid 472 and Alertmanager as uid 65534,
and those names do not resolve inside the helper image.

### Step 4: Start and verify

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml \
  --env-file /opt/zaas/.env start grafana
```

Per-volume checks:

| Volume | Verify |
| ------ | ------ |
| `grafana_data` | Log in at `https://grafana.<domain>`; users and UI-created dashboards are back. Grafana's default plugins are excluded from the archive and its background installer re-downloads them over the next minute - `docker compose logs grafana \| grep "Plugin successfully installed"` |
| `caddy_data` | `docker compose logs caddy` shows certificates loaded from disk, with no new ACME order |
| `alertmanager_data` | Previously active silences are listed at `http://localhost:9093` (from the server) |

---

## Full Server Loss: Rebuild from Offsite

Use this procedure when the host is gone entirely and the only surviving copy is the one on
the backup server (section 12).

### Step 1: Provision a replacement server

```bash
make infra-apply
```

Then work through [Part 1 section 2, Server Bootstrap](#2-server-bootstrap).

### Step 2: Push the archives up to the new host

Run on the **backup server**. Push as the `deploy` user - the offsite reader key is
read-only by design and deliberately cannot write:

```bash
rsync -avz -e "ssh -p 2222" /var/backups/zaas-offsite/ deploy@<new-server-ip>:/var/backups/zaas/
```

### Step 3: Restore PostgreSQL

Follow [PostgreSQL: Restore from pg_dump](#postgresql-restore-from-pg_dump).

### Step 4: Restore the Docker volumes

Follow [Docker Volumes: Restore from tar backup](#docker-volumes-restore-from-tar-backup)
for each of the three volumes. Restoring `caddy_data` first avoids re-issuing certificates
against Let's Encrypt rate limits.

### Step 5: Re-arm the backups

The new host has no timers. Work through [section 9](#9-backups-timers-and-metrics) and
[section 12](#12-offsite-backups-read-only-reader-account) again - the reader account and
its `sshd_config` drop-in do not survive a rebuild, and the host key changed, so re-run
`ssh-keyscan` on the backup server.

Verify the whole chain is back:

```bash
sudo systemctl start zaas-backup.service zaas-backup-volumes.service
curl -s http://localhost:9100/metrics | grep zaas_backup_last_run_success
# -> both backups report 1
```

---

## Observability Stack Bootstrap (data loss)

Use this procedure when Tempo, Prometheus, Loki, or Grafana data is lost (e.g. volumes were deleted). After task 2.1 persistent volumes are in place; this procedure covers a complete re-bootstrap.

**Note:** Metrics, traces, and logs from before the data loss are unrecoverable. This procedure restores the running monitoring system to a working state with a clean slate.

### Step 1: Recreate volumes and restart backends

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d \
  tempo prometheus loki grafana alertmanager
```

Docker Compose will create any missing named volumes automatically.

### Step 2: Verify Grafana provisioning

Open Grafana: `https://grafana.<domain>`

Log in with `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` from `.env`.

Check:
- Datasources (Configuration -> Data Sources): Prometheus, Tempo, and Loki should be present and show "Data source is working".
- Dashboard (Dashboards -> ZaaS Overview) should load without errors (panels will show "No data" until metrics accumulate).

### Step 3: Restart the OTel Collector to reconnect to fresh backends

```bash
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart otel-collector
```

Check Collector logs for successful export confirmations:

```bash
docker logs deploy-otel-collector-1 --since 2m 2>&1 | grep -i "export"
```

### Step 4: Verify metrics are flowing

After 1-2 minutes:
- In Grafana, Prometheus datasource: query `up` - should show all scrape targets.
- In Grafana, Loki datasource: query `{service_name="zaas-api"}` - should show recent API log lines.
- Trigger a test request: `curl https://zaas.at/api/v1/uuid` and check Tempo for the resulting trace.

The observability stack is recovered once all three signals are flowing.
