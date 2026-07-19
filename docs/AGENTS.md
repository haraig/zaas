# Documentation Standards

This file governs documentation for the ZaaS project. It applies to all contributors and AI agents.

> Writing style rules (American English, ASCII punctuation) are defined in the root AGENTS.md and apply to all documentation as well as source code comments.

## Framework: Diataxis

All documentation in this project follows the [Diataxis framework](https://diataxis.fr/). Every document belongs to exactly one of four categories:

| Category | Purpose | Answers the question | Location |
| -------- | ------- | -------------------- | -------- |
| **Tutorial** | Learning-oriented. Guides a newcomer through a concrete task to build understanding. | "How do I learn to do X?" | `docs/tutorials/` |
| **How-to guide** | Task-oriented. Step-by-step instructions for achieving a specific goal. Assumes competence. | "How do I do X?" | `docs/how-to/` |
| **Reference** | Information-oriented. Accurate, complete technical descriptions. No instructions, no explanation. | "What is X?" | `docs/reference/` |
| **Explanation** | Understanding-oriented. Discusses concepts, rationale, and context. No instructions. | "Why is X the way it is?" | `docs/explanation/` |

**Before writing any documentation**, decide which category it belongs to. If it spans multiple categories, split it into multiple documents and cross-link them.

Common mistakes to avoid:
- How-to steps inside a reference file (move the steps to a how-to)
- Rationale prose inside a reference file (move the prose to an explanation)
- Instructions inside a tutorial that are reusable steps (extract to a how-to, link from the tutorial)

## Where to Place New Documentation

| I am writing... | Place it in |
| --------------- | ----------- |
| A guided first experience (learning by doing) | `docs/tutorials/` |
| Steps to accomplish a specific operational task | `docs/how-to/` |
| API/database/config/architecture facts | `docs/reference/` |
| Reasoning behind a design decision | `docs/explanation/` |
| Pitfalls and non-obvious gotchas | `docs/reference/gotchas.md` (append an entry) |
| Manual operational steps (production procedures) | `docs/reference/runbook.md` (append a section) |

## Document Index

### Tutorials

Step-by-step guides for learning by doing.

| File | Description |
| ---- | ----------- |
| [tutorials/add-endpoint.md](tutorials/add-endpoint.md) | Add a new randomness endpoint end-to-end: spec, code generation, service, handler, tests |

### How-to Guides

Task-oriented instructions for specific goals.

| File | Description |
| ---- | ----------- |
| [how-to/run-locally.md](how-to/run-locally.md) | Run the API and full stack locally for development |
| [how-to/deploy.md](how-to/deploy.md) | First-time server bootstrap and automated deploy setup |
| [how-to/infrastructure.md](how-to/infrastructure.md) | Provision Hetzner Cloud server and DNS with OpenTofu |
| [how-to/github-setup.md](how-to/github-setup.md) | Create and configure the GitHub organization and repository: settings, labels, issue/PR workflow, AI code review |

### Reference

Accurate technical descriptions. No instructions.

| File | Description |
| ---- | ----------- |
| [reference/openapi.yaml](reference/openapi.yaml) | OpenAPI 3.0.3 spec - the contractual source of truth for the API |
| [reference/architecture.md](reference/architecture.md) | Package layout, code generation, rate limiting, auth, database, email |
| [reference/observability.md](reference/observability.md) | Tool inventory, architecture diagram, communication ports, Grafana wiring, data storage |
| [reference/runbook.md](reference/runbook.md) | Manual operational procedures: infra setup, email, PostgreSQL, Redis, releases, node exporter |
| [reference/gotchas.md](reference/gotchas.md) | Non-obvious pitfalls and bugs encountered in development |
| [reference/errors/invalid-param.md](reference/errors/invalid-param.md) | INVALID_PARAM error code reference (RFC 9457 type URI) |
| [reference/errors/rate-limited.md](reference/errors/rate-limited.md) | RATE_LIMITED error code reference (RFC 9457 type URI) |
| [reference/errors/invalid-api-key.md](reference/errors/invalid-api-key.md) | INVALID_API_KEY error code reference (RFC 9457 type URI) |
| [reference/errors/internal-error.md](reference/errors/internal-error.md) | INTERNAL_ERROR error code reference (RFC 9457 type URI) |
| [reference/errors/service-unavailable.md](reference/errors/service-unavailable.md) | SERVICE_UNAVAILABLE error code reference (RFC 9457 type URI) |

### Explanation

Conceptual background and design rationale.

| File | Description |
| ---- | ----------- |
| [explanation/design-decisions.md](explanation/design-decisions.md) | Why the API is built the way it is: API-first, response envelope, rate limiter, auth, email |
| [explanation/observability-concepts.md](explanation/observability-concepts.md) | What observability is, why OTel, how the three pillars fit together, application integration |
| [explanation/migration-dynatrace.md](explanation/migration-dynatrace.md) | How to migrate the observability stack to Dynatrace and what would change |

## Updating This Index

When you add, rename, or remove a documentation file, update the table above in the same commit. The index must always reflect the actual state of the `docs/` directory.
