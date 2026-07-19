# Gotchas - Non-obvious Pitfalls Encountered in Development

This document lists real bugs from this codebase that were non-obvious - the kind where knowing the pitfall in advance would have saved time. Routine fixes (typos, style, incremental iteration) are excluded.

Each entry explains the error type, the symptom, why it was non-obvious, and what the fix was.

---

## Docker and Infrastructure

### git refuses operations on directories owned by a different UID


**Symptom:** The webhook redeploy script failed with a confusing git error about repository ownership, even though the directory was correct.

**Gotcha:** Git's CVE-2022-24765 security fix rejects operations on directories owned by a different UID than the current user. A container running as root (UID 0) on a volume cloned by the host user hits this. The error message mentions "dubious ownership" but nothing about Docker UIDs or how to resolve it.

**Fix:** Add `git config --global --add safe.directory /path/to/repo` before any git operation in root containers.

---

### UFW deny-all default silently blocks HTTP/HTTPS


**Symptom:** Server was unreachable after provisioning despite all services running and listening correctly.

**Gotcha:** UFW's default incoming policy is deny-all. Cloud-init scripts that enable UFW without explicitly opening ports 80 and 443 leave the server silently dropping all HTTP traffic. Compounding this: fail2ban does not auto-start on some distributions even when enabled.

**Fix:** Explicitly add `ufw allow 80/tcp` and `ufw allow 443/tcp` in provisioning scripts, and `systemctl start fail2ban` (not just enable).

---

### Hardcoded credentials in Docker Compose default values end up in version control


**Symptom:** A database password was committed in the repository inside a `docker-compose.yaml` default value.

**Gotcha:** Docker Compose `${VAR:-default}` syntax makes it convenient to include full connection strings as defaults (e.g., `${ZAAS_DB_URL:-postgres://user:password@host/db}`). This works locally but the password ends up in version control. The convenience of the pattern actively encourages the mistake.

**Fix:** Never include secrets in default values. Use a `.env.example` file and require explicit configuration.

---

### `set -e` does not propagate into `su -c` subshells


**Symptom:** Bootstrap script silently continued past errors in blocks running as a different user.

**Gotcha:** `su -c "cmd1 && cmd2"` executes in a new subshell. Any `set -e` from the parent script does not apply inside the `su -c` string. Errors are silently swallowed unless you explicitly include `set -e` inside the command string itself.

**Fix:** Write `su -c "set -e; cmd1 && cmd2"` to ensure errors propagate correctly.

---

### OTel Collector needs root to read Docker container logs


**Symptom:** No container logs appeared in the observability stack. The collector started cleanly with no errors.

**Gotcha:** Three independent traps in one:

1. `/var/lib/docker/containers` is mode 700 and owned by root. A non-root collector process silently fails to open any log files.
2. Docker's JSON log timestamps use nanosecond precision. The OTel filelog `%L` strptime layout only handles milliseconds - timestamps fail to parse with `%L`, requiring `%f` for arbitrary fractional seconds.
3. Without `on_error: send` in the operator config, log entries that fail to parse are silently dropped with no indication in collector metrics or logs.

**Fix:** Run the collector as `user: "0"`, use `%f` for fractional seconds in the timestamp layout, and add `on_error: send` to all operators.

---

## OpenTelemetry and Observability

### Port 4317 is gRPC-only; sending HTTP/1.x traces there gives "malformed HTTP response"


**Symptom:** Caddy's trace exporter failed with a "malformed HTTP response" error.

**Gotcha:** The OTLP standard uses two ports: 4317 for gRPC (HTTP/2, binary) and 4318 for HTTP (HTTP/1.x or HTTP/2, JSON or protobuf). Caddy's OTel SDK sends HTTP/1.x. Pointing it at 4317 produces a confusing "malformed HTTP response" error that doesn't clearly indicate a protocol mismatch.

**Fix:** Use port 4318 for any client sending OTLP over HTTP.

---

### The `loki` exporter was silently removed from OTel Collector contrib


**Symptom:** OTel Collector config validation failed after an upgrade; logs stopped reaching Loki with no runtime deprecation warning.

**Gotcha:** The `loki` exporter was removed from the OTel Collector contrib distribution without a deprecation period that emits runtime warnings. The replacement is `otlphttp/loki` with Loki-specific attributes configured differently. An upgrade silently breaks log shipping.

**Fix:** Replace the `loki` exporter with `otlphttp/loki` in the collector pipeline config.

---

### OTel filelog receiver: Docker JSON log timestamp format mismatch

Covered above in the "OTel Collector needs root" entry (`e2bcb39`) - the timestamp format issue (`%L` vs `%f`) is part of the same commit.

---

## Go

### Go nil interface method calls panic at runtime with no compile-time warning


**Symptom:** Auth endpoints panicked at runtime when email was not configured in the environment.

**Gotcha:** When an optional dependency (email sender) is not configured, it is set to `nil`. Go interfaces allow nil values to be assigned and passed without any compile-time error. Calling a method on a nil interface panics at runtime. There is no equivalent to Rust's `Option<T>` forcing explicit handling at the call site.

**Fix:** Guard every use of an optional interface dependency: `if s.emailSender != nil { ... }`.

---

### Redis Lua script results must be bounds-checked before indexing


**Symptom:** Potential panic when indexing the Redis Lua script result slice without bounds checking.

**Gotcha:** Redis Lua scripts return variable-length `[]interface{}` slices. Accessing `res[0]`, `res[1]`, `res[2]` without checking `len(res)` first panics if the script returns fewer values than expected - which can happen due to Lua logic changes, Redis version differences, or error conditions. The type assertions (e.g., `res[0].(int64)`) compound the risk.

**Fix:** Check `len(res)` before indexing. Return an error if the length is unexpected.

---

### oapi-codegen: implementing ServerInterface does not register routes


**Symptom:** New endpoints returned 404 despite handlers being fully implemented and compiling cleanly.

**Gotcha:** When using oapi-codegen with a manual chi router (rather than the generated strict server wrapper), implementing the `ServerInterface` method gives you a compile-time guarantee that the handler *exists* - but if you forget to call `r.Get("/path", handler)` in your router setup, the route is simply never registered. Nothing warns you. The implementation and the routing are two completely separate steps with no link between them.

**Fix:** After implementing any new handler, always update the router registration file. Consider using the generated strict server to make this automatic.

---

### `Content-Transfer-Encoding: quoted-printable` header does not encode the body automatically


**Symptom:** Verification URLs in outgoing emails were corrupted (equals signs mangled). Separately, SMTP delivery to a local Mailpit server failed.

**Gotcha:** Two independent traps:

1. Declaring `Content-Transfer-Encoding: quoted-printable` in a MIME header does not encode the body. You must also pipe the body through `mime/quotedprintable.NewWriter`. The header is a declaration, not an instruction to the library.
2. Go's `smtp.PlainAuth` refuses to send credentials over a non-TLS connection to a non-localhost host. When running Mailpit in Docker with a hostname like `mailpit` rather than `localhost`, it rejects the auth even for a local dev server. Pass `nil` auth when no credentials are configured.

**Fix:** Use `quotedprintable.NewWriter` to encode the body. Pass `nil` as the SMTP auth argument when the username is empty.

---

## Web and Frontend

### Tailwind v4: `hidden` loses to `inline-flex` due to stylesheet order change


**Symptom:** Rate limit warning icons were always visible instead of being hidden by default.

**Gotcha:** In Tailwind v3, `hidden` (which sets `display: none`) had sufficient specificity to win when combined with display utilities in the class list. In Tailwind v4, stylesheet ordering changed - `inline-flex` wins over `hidden` when both are applied. A common pattern of toggling visibility by adding/removing `hidden` alongside a display class breaks silently on upgrade.

**Fix:** Remove `hidden` from the class list entirely. Set `style="display:none"` as the default inline style and toggle via `element.style.display` in JavaScript.

---

### Astro SSG bakes hardcoded domain strings into the build output


**Symptom:** All generated curl examples showed the production domain (`zaas.at`) regardless of the deployment environment.

**Gotcha:** In a static site generator, any string written directly into a page template is rendered at build time and baked into static HTML. There is no runtime substitution. A secondary gotcha: Astro's `define:vars` directive for passing server-side variables does not work in `<script lang="ts">` module scripts - it only works in inline scripts. The value must be passed via a `data-*` attribute on an HTML element and read from the DOM in the script.

**Fix:** Use `PUBLIC_API_BASE_URL` (or equivalent build-time env var), expose it via a `data-api-base-url` attribute on a container element, and read it in the script with `element.dataset.apiBaseUrl`.

---

### Leaflet (and other browser-only libraries) require correct import location in SSG

**Symptom:** Importing Leaflet in an Astro component's frontmatter (`---`) causes the SSG build to fail with a server-side rendering error because `window` and `document` do not exist in the Node.js build context.

**Gotcha:** Astro's SSG phase executes code in the component's frontmatter at build time. Top-level `import 'leaflet'` in the frontmatter fails because Leaflet accesses browser globals. The fix is to import Leaflet inside a `<script>` block (without `is:inline`), which Astro bundles as client-side JavaScript and never executes at build time. Importing CSS from Leaflet (`import 'leaflet/dist/leaflet.css'`) in the frontmatter is fine because Astro/Vite processes CSS statically without executing it.

**Fix:** Place `import L from 'leaflet'` inside the page's `<script>` block, not in the frontmatter. Import the CSS (`import 'leaflet/dist/leaflet.css'`) in the frontmatter so Astro can bundle it as a static asset. No dynamic import wrapper is needed.

---

### Astro static build inlines small component scripts - removing CSP `'unsafe-inline'` breaks them

**Symptom:** After removing `'unsafe-inline'` from `script-src` in the CSP, interactive elements (theme toggle, copy buttons, tool generate buttons) stop working. Browser DevTools shows CSP violations for every page.

**Gotcha:** Astro's static build inlines small, page-specific scripts as `<script type="module">` tags with the JavaScript content embedded directly in the HTML. Per CSP Level 3, `'unsafe-inline'` does not cover inline module scripts - they require a hash (e.g., `'sha256-...'`) or nonce. Since all scripts are inlined with unique, minified content, each page would need its own set of hashes in the CSP header. Caddy serves a single static header, so the hashes must be computed at build time and injected into the Caddyfile or a generated configuration file.

**Fix (current state):** `'unsafe-inline'` is kept in `script-src` for now. This is less secure but required for the site to function.

**Migration path:** Implement a post-build step that extracts all `<script type="module">` inline content from every `.html` file in `dist/`, computes their SHA-256 hashes, and writes a `deploy/Caddyfile.csp` snippet that Caddy imports. Alternatively, use an Astro integration such as `astro-shield` that automates CSP hash generation. See `deploy/Caddyfile` for the `script-src` comment.

---

### VS Code resolves `tsconfig.json` `extends` differently from the CLI


**Symptom:** VS Code showed spurious TypeScript errors that did not appear when running `astro check` on the command line.

**Gotcha:** VS Code's TypeScript language server resolves bare package names in `tsconfig.json` `extends` fields differently from the TypeScript CLI. A value like `"extends": "astro/tsconfigs/strict"` works on the CLI but VS Code fails to find it and falls back to a looser config, producing false errors. The discrepancy only appears in the editor, not in CI, making it hard to diagnose.

**Fix:** Use an explicit relative path in `extends`: `"./node_modules/astro/tsconfigs/strict"`.

---

### OTel SDK resource schema URL conflict when combining WithSchemaURL and WithHost

**Symptom:** `sdkresource.New` returns an error: `conflicting Schema URL: https://opentelemetry.io/schemas/1.26.0 and https://opentelemetry.io/schemas/1.40.0`.

**Gotcha:** When building a custom OTel resource with explicit `WithSchemaURL(semconv.SchemaURL)` (e.g., from `semconv/v1.26.0`) and also adding `WithHost()`, the SDK tries to merge two resources with different schema URLs and fails. The built-in detectors (`WithHost`, `WithProcess`, etc.) use a newer semconv version internally.

**Fix:** Omit `WithSchemaURL` from the `sdkresource.New` call and do not combine it with `WithHost()`. Set resource attributes explicitly via `WithAttributes(...)` only. The schema URL is optional for attribute-only resources.

---
