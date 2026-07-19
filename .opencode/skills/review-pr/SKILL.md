---
description: Review a pull request against the ZaaS project. Use when asked to evaluate, review, or approve a PR. Checks conventional commit format, spec/codegen parity, response envelope helper usage, no hardcoded domains, generated files committed, make all passes.
---

# Review a ZaaS Pull Request

Use this skill whenever you are asked to review, evaluate, or approve a pull request against
the ZaaS repository. Work through every section below in order and record any issues found.

## Reference

OpenAPI spec (source of truth): `docs/reference/openapi.yaml`
Generated code (never hand-edit): `api/internal/gen/openapi.gen.go`
Response helpers: `api/internal/handler/respond.go`
Contribution guide: `CONTRIBUTING.md`
PR template checklist: `.github/PULL_REQUEST_TEMPLATE.md`
Coding conventions: root `AGENTS.md` - Coding Conventions section

## Checklist

Work through these checks in order. Flag any failure with severity: **BLOCKER** (must fix
before merge) or **SUGGESTION** (encouraged but not mandatory).

### 1. Commit message format

- [ ] Every commit follows Conventional Commits: `<type>(<scope>): <description>`
- [ ] Type is one of the accepted list in `CONTRIBUTING.md`:
  `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `style`, `perf`, `chore`
- [ ] Description is in American English, imperative mood, no trailing period
- [ ] No typographic characters in commit messages (no em dashes, curly quotes, ellipsis)

**BLOCKER** if any commit message is malformed - the `commit-msg` lefthook will reject it.

### 2. Spec / codegen parity

- [ ] If `docs/reference/openapi.yaml` was changed, `api/internal/gen/openapi.gen.go` is
  also changed in the same commit (run `make generate` to verify)
- [ ] No hand-edits inside `api/internal/gen/` - these files are always generated
- [ ] The embedded spec copy `api/internal/handler/openapi.yaml` is in sync with
  `docs/reference/openapi.yaml` (both updated by `make generate`)

**BLOCKER** if spec was changed without regenerating, or if generated files were hand-edited.

### 3. Response envelope helpers

For every new or changed handler method in `api/internal/handler/server.go`:

- [ ] Single-value responses use `WriteSingle(w, r, result)`
- [ ] Slice responses use `WriteMultiple(w, r, results)`
- [ ] Error responses use `WriteError(w, statusCode, errorCode, message)` with an error
  code constant from `gen` - no raw `http.Error` or inline JSON writes

**BLOCKER** if a handler writes a response without the helpers (breaks the response envelope
contract and RFC 9457 error shape).

### 4. No hardcoded domains or base URLs

- [ ] No string literals containing domain names (e.g., `zaas.example.com`, `localhost:8080`
  used as a configurable value rather than a test fixture)
- [ ] `ZAAS_BASE_URL` is the single source of truth for the service domain; config is loaded
  from environment only

**BLOCKER** if a domain is hardcoded in non-test production code.

### 5. Security: randomness and secrets

- [ ] Any new password, token, key, or secret generation uses `crypto/rand`, never
  `math/rand`
- [ ] No `.env` files or secret values committed (check `git diff --name-only`)

**BLOCKER** if `math/rand` is used for security-sensitive output.

### 6. Test coverage

- [ ] New service functions have a `_test.go` companion in `api/internal/service/`
- [ ] New handler methods have a `_test.go` companion in `api/internal/handler/`
- [ ] Changed behavior has corresponding updated or new test cases

**SUGGESTION** if tests are missing for non-trivial logic.

### 7. `make all` passes

Ask the author (or run locally):

```bash
make all   # fmt + vet + lint + mod-tidy + test
```

If web files were changed:

```bash
make web-check   # astro check + web build
```

**BLOCKER** if `make all` or `make web-check` fails.

### 8. Writing style

- [ ] All code comments and documentation in American English
- [ ] No typographic characters: no em dashes (`--`), no curly quotes, no ellipsis
  character, no middle dot - use plain ASCII instead
- [ ] No hardcoded "Phase N" labels leaked into user-visible text or docs

**SUGGESTION** for style issues that do not affect behavior.

### 9. PR template completeness

- [ ] Author has completed every item in `.github/PULL_REQUEST_TEMPLATE.md`
- [ ] PR description explains the "why", not just the "what"

**SUGGESTION** if template items are skipped without explanation.

## Verdict

After completing all checks, summarize:

```
BLOCKERS: <count> - list each one with file:line reference
SUGGESTIONS: <count> - list each one
OVERALL: APPROVE / REQUEST CHANGES
```

Request changes if there is at least one BLOCKER. Approve only when all BLOCKERs are resolved.
