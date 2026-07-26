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
| Webhook | Receives deploy triggers from GitHub Actions. Built locally from `deploy/webhook/Dockerfile` (not pulled from a registry) - see step 4. |

## Prerequisites

- A server with Docker and the Docker Compose plugin installed, and `git`
  (both are already present if provisioned via [infrastructure.md](infrastructure.md) on the
  `ubuntu-26.04` Hetzner image - Docker is installed by cloud-init, and `git` ships with the base image)
- DNS A records pointing to your server for `$ZAAS_DOMAIN` and `grafana.$ZAAS_DOMAIN`
- Ports 80 and 443 open (Caddy handles TLS via Let's Encrypt)

## Initial Server Bootstrap

This is a one-time setup. After this, every push to `main` deploys automatically.

**1. Create a dedicated deploy user:**

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin --groups docker deploy
sudo mkdir -p /opt/zaas
sudo chown deploy:deploy /opt/zaas
```

**2. Clone the repository (sparse checkout - `deploy/` folder only):**

```bash
sudo su -s /bin/sh deploy -c '
  set -e
  git clone --filter=blob:none --sparse https://github.com/haraig/zaas.git /opt/zaas
  git -C /opt/zaas sparse-checkout set deploy
'
```

**3. Create the `.env` file:**

```bash
sudo su -s /bin/sh deploy -c '
  cp /opt/zaas/.env.example /opt/zaas/.env
  chmod 600 /opt/zaas/.env
'
```

The `chmod 600` restricts the file to the `deploy` user only, since it will hold secrets (`GRAFANA_ADMIN_PASSWORD`, `DEPLOY_WEBHOOK_SECRET`).

Edit `/opt/zaas/.env` and set at minimum:

```dotenv
ZAAS_DOMAIN=your-domain.example
GRAFANA_ADMIN_PASSWORD=<strong-password>
DEPLOY_WEBHOOK_SECRET=<generate with: openssl rand -hex 32>
```

> The `DEPLOY_WEBHOOK_SECRET` must match the `DEPLOY_WEBHOOK_SECRET` secret set in GitHub Actions.

> `PUBLIC_API_BASE_URL`, `PUBLIC_IMPRINT_NAME`, `PUBLIC_IMPRINT_ADDRESS`, `PUBLIC_IMPRINT_EMAIL`, and
> `PUBLIC_PRIVACY_EMAIL` are **not** read from this file. The web image is a prebuilt static site
> (`output: "static"` in Astro) - these values are baked in at CI build time from GitHub Actions
> variables (see [GitHub Actions Configuration](#github-actions-configuration) below), and the
> `caddy` service never receives them as container environment variables. Setting them here has no
> effect in production.

**4. Start the full stack:**

```bash
sudo su -s /bin/sh deploy -c '
  set -e
  cd /opt/zaas
  docker compose -f /opt/zaas/deploy/docker-compose.yaml --env-file /opt/zaas/.env up -d
'
```

Unlike the other services, `webhook` has no `image:` to pull - it's built locally from `deploy/webhook/Dockerfile`, so this first `up -d` also builds it (needs outbound network access for `apk add`). Confirm it built with the tools `redeploy.sh` needs:

```bash
docker exec deploy-webhook-1 git --version
docker exec deploy-webhook-1 docker --version
```

Caddy provisions TLS certificates automatically on first start.

**5. Verify:**

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

> `redeploy.sh` only recreates `api` and `caddy` (`--no-deps api caddy`). Changes to the `webhook` service itself - `deploy/webhook/hooks.json`, `redeploy.sh`, `deploy/webhook/Dockerfile`, or its `docker-compose.yaml` block - are **not** applied automatically, even after `git pull` fetches them. `docker compose up -d` also won't recreate a running container just because a bind-mounted file's contents changed (only config diffs trigger recreation). After such a change, rebuild and force-restart it manually on the server:
>
> ```bash
> cd /opt/zaas
> docker compose -f deploy/docker-compose.yaml --env-file .env build webhook
> docker compose -f deploy/docker-compose.yaml --env-file .env up -d --force-recreate --no-deps webhook
> ```

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
| `PUBLIC_PRIVACY_EMAIL` | Email for GDPR data subject requests (falls back to `PUBLIC_IMPRINT_EMAIL` if unset) |

## Branch Protection

To ensure deploys only run after CI passes:

1. Go to **Settings -> Branches -> Add rule** for `main`
2. Enable **Require status checks to pass before merging**
3. Add `Go - fmt / vet / lint / test`, `Web - lint + build`, and `Docker - build smoke tests` as required checks
