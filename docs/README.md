# Documentation

This project follows the [Diátaxis](https://diataxis.fr/) documentation framework, organizing docs into four categories:

## Tutorials

Step-by-step learning exercises for newcomers.

- [Add a new endpoint](tutorials/add-endpoint.md) - build a complete API endpoint from spec to deployment

## How-To Guides

Task-oriented instructions for specific goals.

- [Run locally](how-to/run-locally.md) - start the API and full stack on your machine
- [Deploy](how-to/deploy.md) - server bootstrap and automated deploy setup
- [Infrastructure](how-to/infrastructure.md) - provision Hetzner Cloud with OpenTofu

## Reference

Technical descriptions of the system as it is.

- [Architecture](reference/architecture.md) - package layout, code generation, rate limiting, auth, DB
- [OpenAPI spec](reference/openapi.yaml) - source of truth for the API contract
- [Observability](reference/observability.md) - tool inventory, ports, pipelines, Grafana wiring
- [Runbook](reference/runbook.md) - manual operational procedures
- [Gotchas](reference/gotchas.md) - non-obvious pitfalls encountered in development

## Explanation

Background and context for understanding design choices.

- [Design decisions](explanation/design-decisions.md) - why the API is built the way it is
- [Observability concepts](explanation/observability-concepts.md) - OTel concepts, push/pull, application integration
- [Migration to Dynatrace](explanation/migration-dynatrace.md) - how to migrate the observability stack
