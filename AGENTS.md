# AGENTS.md - AI Assistant Configuration

## Project

**ZaaS** (Zufall as a Service) - an open-source REST API providing randomness-as-a-service. Roll dice, generate UUIDs, pick random colors, get random coordinates, generate passwords, random words, lorem ipsum, SSH keypairs, and more. Free API keys available for higher rate limits.

**Tech stack:**

- API: Go 1.26+, chi router, oapi-codegen, OpenTelemetry
- Web: Astro, Tailwind CSS v4, Scalar API playground
- Deployment: Docker, Caddy, Hetzner Cloud
- CI/CD: GitHub Actions, Conventional Commits, git-cliff

## Commands

```bash
make setup      # Install golangci-lint, npm deps, lefthook
make dev        # Start full stack via docker compose
make dev-minimal # Start minimal stack: API + web + Caddy only (no DB, Redis, observability)
make stop       # Stop the local dev stack (keep volumes)
make test       # Run Go tests (cd api && go test ./...)
make lint       # Run golangci-lint on Go code
make generate   # Regenerate Go types from OpenAPI spec
make build      # Build API Docker image
make web-dev    # Start Astro dev server
make web-build  # Build Astro static site
make web-build-docker # Build Astro static site inside Docker
make web-lint   # Run astro check (TypeScript diagnostics)
make web-check  # web-lint + web-build
make fmt        # Check Go formatting
make fmt-fix    # Auto-fix Go formatting
make fmt-web    # Check web formatting (Prettier)
make fmt-web-fix  # Auto-fix web formatting (Prettier)
make fmt-all    # Check all formatting (Go + web)
make fmt-all-fix  # Auto-fix all formatting (Go + web)
make vet        # Run go vet
make mod-tidy   # Run go mod tidy
make all        # fmt + vet + lint + mod-tidy + test
make infra-init    # tofu init
make infra-plan    # tofu plan
make infra-apply   # tofu apply   # DESTRUCTIVE - mutates Hetzner Cloud; requires explicit user approval
make infra-destroy # tofu destroy # DESTRUCTIVE - deletes the production server; requires explicit user approval
make clean      # Stop containers, remove volumes
make distclean  # Stop containers, remove volumes, delete node_modules/build artifacts
```

Run the API locally without Docker:

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server
```

## Repository Structure

```
zaas/
├── api/                        # Go REST API service
│   ├── AGENTS.md               # API-specific AI assistant config
│   ├── cmd/server/             # Entrypoint (main.go)
│   ├── internal/
│   │   ├── config/             # Env var config loading
│   │   ├── gen/                # oapi-codegen generated types + ServerInterface
│   │   ├── handler/            # HTTP handlers (server.go implements ServerInterface)
│   │   ├── middleware/         # CORS, logging, rate limiting, request ID
│   │   ├── ratelimiter/        # RateLimiter interface + MemoryRateLimiter
│   │   ├── service/            # Business logic (dice, coin, number, uuid, color, coordinates)
│   │   ├── email/              # Email sending (API key registration flow)
│   │   ├── store/              # Database access layer
│   │   └── telemetry/          # OpenTelemetry SDK initialization
│   ├── Dockerfile
│   └── go.mod
├── web/                        # Astro homepage + tool pages
│   ├── AGENTS.md               # Web-specific AI assistant config
│   ├── src/
│   │   ├── pages/              # Routes: /, /docs, /tools/*, /imprint, /privacy, /datenschutz
│   │   ├── layouts/            # BaseLayout with theme system
│   │   ├── components/         # Shared UI components
│   │   └── styles/             # Tailwind v4 CSS
│   ├── astro.config.mjs
│   └── package.json
├── infra/                      # Infrastructure-as-code
│   ├── AGENTS.md               # Infra-specific AI assistant config
│   └── tofu/                   # OpenTofu config for Hetzner Cloud
├── deploy/                     # Production deployment stack
│   ├── docker-compose.yaml     # Production Docker Compose
│   ├── docker-compose.local.yaml # Local Docker Compose override
│   ├── Caddyfile               # Production Caddy reverse proxy config
│   ├── Caddyfile.local         # Local Caddy config
│   ├── grafana/                # Grafana dashboards and datasources
│   ├── scripts/                # Deploy and backup scripts
│   ├── webhook/                # Webhook-based redeploy config
│   └── *.yaml                  # Observability stack configs (Loki, Tempo, Prometheus, OTel)
├── docs/
│   ├── AGENTS.md               # Documentation standards, Diataxis rules, full doc index
│   ├── tutorials/
│   │   └── add-endpoint.md     # Tutorial: add a new API endpoint end-to-end
│   ├── how-to/
│   │   ├── run-locally.md      # Run the API and full stack locally
│   │   ├── deploy.md           # Server bootstrap and automated deploy setup
│   │   └── infrastructure.md   # Provision Hetzner Cloud with OpenTofu
│   ├── reference/
│   │   ├── openapi.yaml        # OpenAPI 3.0.3 spec (source of truth)
│   │   ├── architecture.md     # Package layout, code generation, rate limiting, auth, DB
│   │   ├── observability.md    # Tool inventory, ports, pipelines, Grafana wiring
│   │   ├── runbook.md          # Manual operational procedures
│   │   └── gotchas.md          # Non-obvious pitfalls encountered in development
│   └── explanation/
│       ├── design-decisions.md         # Why the API is built the way it is
│       ├── observability-concepts.md   # OTel concepts, push/pull, application integration
│       └── migration-dynatrace.md      # How to migrate the observability stack to Dynatrace
├── .env.example                # All env vars documented with defaults
├── Makefile                    # Developer convenience targets
└── README.md
```

## Coding Conventions

- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) - enforced by commitlint via lefthook. Accepted types: `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `style`, `perf`, `chore`. See `CONTRIBUTING.md` for the full list with examples.
- **Language:** All code, docs, comments, and configuration in American English
- **Writing style:** Use American English spelling throughout all documentation and source code comments. Avoid typographic characters that look out of place in code projects: no em dashes (--), no en dashes, no curly quotes, no ellipsis character, no middle dot, no guillemets. Use plain ASCII punctuation instead (e.g., " - " for a dash, "..." for ellipsis, straight quotes).
- **Config:** Environment variables only. No hardcoded domains - `ZAAS_BASE_URL` is the single source of truth
- **API-first:** `docs/reference/openapi.yaml` is the source of truth. Run `make generate` after spec changes
- **Documentation:** Follow the [Diataxis framework](https://diataxis.fr/) for all documentation. See `docs/AGENTS.md` for category definitions, placement rules, and the full document index.
- **Domain-specific conventions:** See `api/AGENTS.md`, `web/AGENTS.md`, `infra/AGENTS.md`

## Guardrails

These are absolute prohibitions. Do not do any of the following under any circumstances:

- **NEVER edit files under `api/internal/gen/`** - they are generated by `make generate` and any hand-edit will be overwritten.
- **NEVER use `math/rand` for keys, passwords, or tokens** - always use `crypto/rand`. This is a randomness service; use cryptographically secure randomness everywhere security matters.
- **NEVER commit `.env` files or secrets** - see the "Never commit" list below.
- **NEVER run `make infra-apply` or `make infra-destroy`** without explicit user approval in this session. These commands mutate or destroy the production Hetzner Cloud server.
- **NEVER modify `deploy/Caddyfile`, `deploy/docker-compose.yaml`, or files under `.github/workflows/`** without explicit user instruction. These control production deployments.
- **NEVER hardcode domain names or base URLs** - `ZAAS_BASE_URL` is the single source of truth for the service domain.
- **NEVER bypass git hooks** with `--no-verify`. Fix the underlying issue instead.

## Never commit

The following must never be staged or committed:

- `.env` (any variant - use `.env.example` as the template)
- `node_modules/`
- `web/dist/` and `web/.astro/`
- `*.tfstate` and `*.tfstate.backup`
- Build artifacts (`api/server`, compiled binaries)

## Pre-commit / pre-push hooks

lefthook runs these checks automatically:

| Stage | Checks | How to fix |
| ----- | ------ | ---------- |
| `commit-msg` | commitlint (Conventional Commits format) | Fix the commit message format; see `CONTRIBUTING.md` for accepted types |
| `pre-commit` | `gofmt`, `go vet` | Run `make fmt-fix` then `make vet` |
| `pre-push` | `golangci-lint`, `go test ./...` | Run `make lint` then `make test` |

If a hook rejects your commit, **fix the underlying issue** - do not use `--no-verify`.

Run `make setup` once after cloning to install the hooks via lefthook.

## Key References

- OpenAPI spec: `docs/reference/openapi.yaml` - source of truth for the API contract
- Architecture reference: `docs/reference/architecture.md`
- Design decisions: `docs/explanation/design-decisions.md`
- Operations runbook: `docs/reference/runbook.md` - **add manual steps here, not in plans**
- Observability reference: `docs/reference/observability.md`
- Observability concepts: `docs/explanation/observability-concepts.md`
- Documentation standards and index: `docs/AGENTS.md`
- Environment variables: `.env.example` - all env vars documented with defaults
- PR checklist: `.github/PULL_REQUEST_TEMPLATE.md` - complete every item before opening a PR
- Commit types and contribution guide: `CONTRIBUTING.md`

## Available Skills

Skills provide specialized instructions and workflows for specific tasks.
Use the skill tool to load a skill when a task matches its description.

| Skill | Trigger | Location |
| ----- | ------- | -------- |
| `add-endpoint` | Add a new randomness endpoint end-to-end (spec, codegen, service, handler, tests) | `.claude/skills/add-endpoint/SKILL.md` |
| `update-spec` | Modify an existing endpoint: add/remove parameters, change response shapes, rename fields | `.claude/skills/update-spec/SKILL.md` |
| `review-pr` | Review or approve a pull request against ZaaS (commit format, codegen parity, helpers, security) | `.claude/skills/review-pr/SKILL.md` |
