# Contributing to ZaaS

Thank you for your interest in contributing!

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold it.

## Reporting Bugs and Requesting Features

- **Bugs:** Open an issue using the [Bug Report template](.github/ISSUE_TEMPLATE/).
- **Features:** Open an issue using the [Feature Request template](.github/ISSUE_TEMPLATE/).
- **Security vulnerabilities:** See [SECURITY.md](SECURITY.md) - do not open a public issue.

## Prerequisites

- Go 1.26+
- Node.js 22+
- Docker
- [lefthook](https://get.lefthook.com)
- [git-cliff](https://git-cliff.org)

## Setup

```bash
git clone https://github.com/haraig/zaas.git
cd zaas
make setup   # installs golangci-lint, npm deps, and lefthook git hooks
```

`make setup` installs lefthook, which registers `pre-commit` and `pre-push` git hooks. These run automatically and will reject commits or pushes that do not pass formatting, vetting, linting, or tests. **Do not bypass hooks with `--no-verify`** - fix the underlying issue instead.

## Style Rules

- **Language:** American English throughout all code, comments, and documentation.
- **Punctuation:** Plain ASCII only - no Unicode arrows, em dashes, curly quotes, middle dots, or similar. See the style section in [AGENTS.md](AGENTS.md) for the full list.
- **Go formatting:** Enforced by `gofmt` (run `make fmt-fix` to auto-fix).
- **Web formatting:** Enforced by Prettier (run `make fmt-web-fix` to auto-fix).

## Development

```bash
make dev          # start the full local stack (API + web + observability)
make test         # run Go tests
make lint         # run golangci-lint
make web-dev      # start the Astro dev server
make fmt-all-fix  # auto-fix formatting (run before committing)
```

Run the API without Docker:

```bash
ZAAS_OTEL_ENABLED=false go run ./api/cmd/server
```

## Making Changes

### API changes

The OpenAPI spec (`docs/reference/openapi.yaml`) is the source of truth. If you change the spec, regenerate Go types:

```bash
make generate
```

Do not manually edit files in `api/internal/gen/` - they are generated.

### Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/). The format is enforced by commitlint via lefthook on every `git commit`.

```
feat: add shuffle endpoint
fix: correct dice roll distribution
docs: update configuration table
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `style`, `perf`, `chore`

## Before Opening a PR

Run the full validation pipeline locally and ensure it passes:

```bash
make all       # fmt + vet + lint + mod-tidy + test
make web-check # TypeScript check + web build
```

Also review the [PR template](.github/PULL_REQUEST_TEMPLATE.md) and complete every checklist item before submitting.

## Pull Requests

1. Fork the repository and create a branch from `main`
2. Make your changes with passing tests and linting
3. Open a PR against `main` - the PR template will guide you

CI runs automatically on every PR. All checks must pass before merging.

## Questions

Open a [GitHub Discussion](https://github.com/haraig/zaas/discussions) for questions or ideas.
