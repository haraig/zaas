# How to Deploy ZaaS

This guide covers first-time server setup and explains how automated deploys work after that.

> For infrastructure provisioning (creating the server on Hetzner), see [infrastructure.md](infrastructure.md).
> For detailed operational procedures (email setup, database, backups), see [../reference/runbook.md](../reference/runbook.md).

## Stack

| Service | Role |
| ------- | ---- |
| API | Go REST API (built from `api/`) |
| Caddy | Reverse proxy, TLS termination, static web |
| OTel Collector | Collects traces, metrics, and logs |
| Prometheus | Metrics storage |
| Tempo | Distributed trace storage |
| Loki | Log storage |
| Grafana | Observability dashboards |
| Webhook | Receives deploy triggers from GitHub Actions |

## Prerequisites

- A server with Docker and the Docker Compose plugin installed
- DNS A records pointing to your server for `$ZAAS_DOMAIN` and `grafana.$ZAAS_DOMAIN`
- Ports 80 and 443 open (Caddy handles TLS via Let's Encrypt)

## Initial Server Bootstrap

This is a one-time setup. After this, every push to `main` deploys automatically.

**1. SSH into the server and install prerequisites:**

```bash
ssh root@<server-ip>
apt-get update && apt-get install -y docker.io docker-compose-plugin git
systemctl enable --now docker
```

**2. Create a dedicated deploy user:**

```bash
useradd --system --no-create-home --shell /usr/sbin/nologin --groups docker deploy
mkdir -p /opt/zaas
chown deploy:deploy /opt/zaas
```

**3. Clone the repository (sparse checkout - `deploy/` folder only):**

```bash
su -s /bin/sh deploy -c '
  set -e
  git clone --filter=blob:none --sparse https://github.com/haraig/zaas.git /opt/zaas
  git -C /opt/zaas sparse-checkout set deploy
'
```

**4. Create the `.env` file:**

```bash
su -s /bin/sh deploy -c 'cp /opt/zaas/.env.example /opt/zaas/.env'
```

Edit `/opt/zaas/.env` and set at minimum:

```dotenv
ZAAS_DOMAIN=your-domain.example
GRAFANA_ADMIN_PASSWORD=<strong-password>
DEPLOY_WEBHOOK_SECRET=<generate with: openssl rand -hex 32>

# Required for imprint/privacy pages (Austrian ECG section 5)
PUBLIC_IMPRINT_NAME=Your Name
PUBLIC_IMPRINT_ADDRESS=Street 1\nCity, Country
PUBLIC_IMPRINT_EMAIL=contact@example.com
```

> The `DEPLOY_WEBHOOK_SECRET` must match the `DEPLOY_WEBHOOK_SECRET` secret set in GitHub Actions.

**5. Start the full stack:**

```bash
su -s /bin/sh deploy -c '
  set -e
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d
'
```

Caddy provisions TLS certificates automatically on first start.

**6. Verify:**

```bash
curl https://your-domain.example/healthz
# -> {"status":"ok"}
```

## Automated Deploys

From this point on, every push to `main` that passes CI will:

1. Build new API and web Docker images
2. Push them to GHCR (tagged with short SHA + `latest`)
3. Trigger a rolling restart on the server via the deploy webhook

Changes to `deploy/` configs (Caddyfile, Grafana dashboards, etc.) are picked up via `git pull` in the deploy script.

## GitHub Actions Configuration

Configure these in **Settings -> Secrets and variables -> Actions**:

| Secret | Description | How to generate |
| ------ | ----------- | --------------- |
| `DEPLOY_WEBHOOK_SECRET` | Bearer token for the deploy webhook | `openssl rand -hex 32` |

`GITHUB_TOKEN` is provided automatically - no setup needed.

Additionally, configure these **Actions variables** for the web build:

| Variable | Description |
| -------- | ----------- |
| `PUBLIC_API_BASE_URL` | Public API base URL (e.g. `https://your-domain.example`) |
| `PUBLIC_IMPRINT_NAME` | Legal name for imprint page |
| `PUBLIC_IMPRINT_ADDRESS` | Postal address (use `\n` as line separator) |
| `PUBLIC_IMPRINT_EMAIL` | Contact email for imprint page |

## Branch Protection

To ensure deploys only run after CI passes:

1. Go to **Settings -> Branches -> Add rule** for `main`
2. Enable **Require status checks to pass before merging**
3. Add `Go - fmt / vet / lint / test`, `Web - lint + build`, and `Docker - build smoke tests` as required checks
