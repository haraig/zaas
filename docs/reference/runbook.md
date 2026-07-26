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
9. [PostgreSQL: Backup Timer Activation](#9-postgresql-backup-timer-activation)
10. [Redis: Production .env Configuration](#10-redis-production-env-configuration)
11. [Node Exporter: Host Metrics](#11-node-exporter-host-metrics)

### Part 2: Operational Procedures

Ad-hoc and ongoing procedures, looked up as needed.

- [Releases](#releases)
- [DMARC Policy Tightening](#dmarc-policy-tightening)
- [Issuing an API Key on Request](#issuing-an-api-key-on-request)
- [PostgreSQL: Manual Database Access](#postgresql-manual-database-access)
- [Client Administration (SQL Reference)](#client-administration-sql-reference)

### Part 3: Alert Playbooks

Procedures for responding to Prometheus alerts. Each section corresponds to an alert rule in `deploy/prometheus.rules.yaml`.

- [First Response Checklist](#first-response-checklist)
- [ZaasApiDown](#zaas-api-down)
- [ZaasHighErrorRate](#zaas-high-error-rate)
- [ZaasHighP99Latency](#zaas-high-p99-latency)
- [ZaasHighRateLimitRate](#zaas-high-rate-limit-rate)
- [ZaasDiskSpaceLow / ZaasDiskSpaceCritical](#zaas-disk-space-low)
- [ZaasPostgresDown](#zaas-postgres-down)
- [ZaasRedisDown](#zaas-redis-down)
- [ZaasCollectorDroppedSpans / ZaasCollectorDroppedMetrics / ZaasCollectorDroppedLogs / ZaasCollectorDown](#zaas-collector-dropped-data)
- [PostgreSQL: Restore from pg_dump](#postgresql-restore-from-pg_dump)
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

---

## 2. Server Bootstrap

**Source:** `docs/how-to/deploy.md`

The server image (`docker-ce`) already has Docker installed and running. Cloud-init runs `package_update` and `package_upgrade` on first boot, and disables root login. SSH in as the configured user on the custom SSH port (default: `2222`):

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

## 9. PostgreSQL: Backup Timer Activation

### 9.1 Create backup directory

```bash
sudo mkdir -p /var/backups/zaas
sudo chown deploy:deploy /var/backups/zaas
```

### 9.2 Install and enable the systemd timer

```bash
sudo cp /opt/zaas/deploy/scripts/zaas-backup.service /etc/systemd/system/
sudo cp /opt/zaas/deploy/scripts/zaas-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zaas-backup.timer
```

### 9.3 Verify

```bash
systemctl status zaas-backup.timer
# -> Active: active (waiting)

# Test a manual run:
sudo systemctl start zaas-backup.service
journalctl -u zaas-backup.service --no-pager -n 20
ls -la /var/backups/zaas/daily/
```

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
http://<server-ip>:9090/targets
# -> redis job should show "UP"
```

---

## 11. Node Exporter: Host Metrics

Node Exporter exposes host-level metrics (CPU, memory, disk, network, filesystem) to Prometheus. It runs as an unprivileged systemd service on the host and is scraped by the Prometheus container via `host.docker.internal:9100`.

### 11.1 Download and install

```bash
# Download latest stable release
NODE_EXPORTER_VERSION="1.9.1"
wget "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64.tar.gz" \
  -O /tmp/node_exporter.tar.gz

tar -xzf /tmp/node_exporter.tar.gz -C /tmp/
install -m 755 /tmp/node_exporter-${NODE_EXPORTER_VERSION}.linux-amd64/node_exporter /usr/local/bin/node_exporter
rm -rf /tmp/node_exporter*
```

### 11.2 Create dedicated system user

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin node_exporter
```

### 11.3 Create systemd service unit

```bash
sudo cat > /etc/systemd/system/node_exporter.service << 'EOF'
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
  --web.listen-address=:9100
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
```

> **Why `--collector.disable-defaults` with explicit collectors?** This avoids hundreds of low-value metrics (systemd units, NFS, hardware sensors, etc.) and keeps Prometheus cardinality low. The selected collectors cover all the metrics used by the Grafana dashboard.

### 11.4 Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now node_exporter
```

### 11.5 Verify

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
# http://<server-ip>:9090/targets  ->  node-exporter job should show "UP"
```

### 11.6 Firewall note

Node Exporter binds to all interfaces by default. UFW is enabled by cloud-init; ensure port 9100 is not exposed externally - it only needs to be reachable from the Docker bridge network:

```bash
ufw status
# If port 9100 would be open to the internet, restrict it:
sudo ufw deny 9100/tcp
# The Docker bridge network bypasses UFW via iptables, so Prometheus can
# still reach the host on host.docker.internal:9100.
```

---

# Part 2: Operational Procedures

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

# Part 3: Alert Playbooks

This part documents the response procedure for each Prometheus alert defined in `deploy/prometheus.rules.yaml`. Alert notifications are delivered via Alertmanager (configured in `deploy/alertmanager.yaml`). The Alertmanager UI is at `http://<server-ip>:9093`.

## First Response Checklist

When paged, run through these steps before diving into a specific alert playbook:

1. Open Grafana ZaaS Overview: `http://<server-ip>:3000` (or `https://zaas.at/grafana` if proxied). Check for red panels.
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
8. If the Grafana dashboard shows no data at all, check the OTel Collector (see [ZaasCollectorDown](#zaas-collector-dropped-data)).
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

# Attempt a health check directly
docker exec deploy-api-1 wget -qO- http://localhost:8080/healthz
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

# Check the current rate limit configuration
docker exec deploy-api-1 env | grep ZAAS_RATE_LIMIT
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

**Diagnostic steps:**

```bash
# Find largest consumers
df -h /
du -sh /var/lib/docker/*
du -sh /var/backups/zaas/

# Check Docker disk usage
docker system df
```

**Remediation:**

```bash
# Remove unused Docker images and containers
docker system prune -f

# Remove old backup files (keep at least 7 days)
ls -lt /var/backups/zaas/daily/
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
# http://<server-ip>:9090/graph?g0.expr=otelcol_exporter_send_failed_spans_total
```

**Remediation:**

```bash
# If a backend is down, restart it first, then restart the Collector
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart tempo
sudo -u deploy docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env restart otel-collector

# If the Collector config was recently changed and it fails to start, revert the change
```

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
  | docker exec -i deploy-postgres-1 psql -U zaas -d zaas
```

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

Open Grafana: `http://<server-ip>:3000`

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
