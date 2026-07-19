GOLANGCI_LINT_VERSION := v2.12.2

.PHONY: all setup generate dev dev-minimal stop test fmt fmt-fix vet mod-tidy lint build web-build-docker clean distclean \
        web-dev web-build web-lint web-check \
        fmt-web fmt-web-fix fmt-all fmt-all-fix \
        infra-init infra-plan infra-apply infra-destroy

# Run the full Go pipeline
all: fmt vet lint mod-tidy test

# ── Setup ──

# Install dev tools and git hooks
setup:
	cd api && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	npm install
	npm install --prefix web
	lefthook install

# Regenerate Go types from OpenAPI spec
generate:
	cp docs/reference/openapi.yaml api/internal/handler/openapi.yaml
	cd api && go generate ./internal/gen/...

# ── Development ──

# Start full stack via Docker Compose
dev:
	POSTGRES_PASSWORD=$${POSTGRES_PASSWORD:-zaas} docker compose -f deploy/docker-compose.yaml -f deploy/docker-compose.local.yaml up

# Start minimal stack: API + web + Caddy only (no DB, Redis, or observability)
dev-minimal:
	docker compose -f deploy/docker-compose.minimal.yaml up

# Stop the local dev stack (no volume removal)
stop:
	POSTGRES_PASSWORD=$${POSTGRES_PASSWORD:-zaas} docker compose -f deploy/docker-compose.yaml -f deploy/docker-compose.local.yaml down

# Start Astro dev server
web-dev:
	cd web && npm run dev

# ── Go pipeline ──

# Check formatting (does not modify files; run fmt-fix to apply)
fmt:
	@cd api && gofmt -s -l . | grep . && echo "ERROR: unformatted files above - run: make fmt-fix" && exit 1 || true

# Auto-fix formatting
fmt-fix:
	cd api && gofmt -s -w .

# Check formatting of web/docs files with Prettier (does not modify files)
fmt-web:
	cd web && npx prettier --check ".." --ignore-path ../.prettierignore

# Auto-fix formatting of web/docs files with Prettier
fmt-web-fix:
	cd web && npx prettier --write ".." --ignore-path ../.prettierignore

# Check formatting of all files (Go + web/docs)
fmt-all: fmt fmt-web

# Auto-fix formatting of all files (Go + web/docs)
fmt-all-fix: fmt-fix fmt-web-fix

# Catch common mistakes (unreachable code, bad printf args, etc.)
vet:
	cd api && go vet ./...

# Tidy go.mod/go.sum and fail if they are dirty afterwards
mod-tidy:
	cd api && go mod tidy
	@git diff --exit-code api/go.mod api/go.sum || \
		(echo "ERROR: go.mod/go.sum changed after mod tidy - commit them" && exit 1)

# Run all configured linters
lint:
	cd api && golangci-lint run ./...

# Run tests
test:
	cd api && go test ./...

# ── Build ──

# Build API Docker image
# The API binary embeds the version from the latest git tag via -ldflags.
build:
	docker build \
		--build-arg VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
		-t zaas-api:local api/

# Build Web (Caddy + Astro) Docker image
web-build-docker:
	docker build -t zaas-web:local web/

# Build Astro static site
web-build:
	cd web && npm run build

# ── Web QA ──

# Run Astro TypeScript diagnostics
web-lint:
	cd web && npx astro check

# Lint + build the web frontend
web-check: web-lint web-build

# ── Cleanup ──

# Stop containers and remove volumes
clean:
	POSTGRES_PASSWORD=$${POSTGRES_PASSWORD:-zaas} docker compose -f deploy/docker-compose.yaml -f deploy/docker-compose.local.yaml down -v

# Remove all generated/cached build artifacts (node_modules, Astro cache, dist)
distclean: clean
	rm -rf web/node_modules web/.astro web/dist
	rm -rf node_modules
	rm -rf infra/tofu/.terraform

# ── Infrastructure ──

infra-init:
	tofu -chdir=infra/tofu init

infra-plan:
	tofu -chdir=infra/tofu plan

infra-apply:
	tofu -chdir=infra/tofu apply

infra-destroy:
	tofu -chdir=infra/tofu destroy
