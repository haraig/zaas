/**
 * Shared tool-runner for ZaaS tool pages.
 *
 * Wires up the generate button, calls the API, updates the API-call panel,
 * and reflects rate-limit headers. Each tool page provides the page-specific
 * parts via RunToolOptions.
 */

export interface RunToolOptions {
  /** ID of the element that wraps the result (shown/hidden on success). */
  resultContainerId: string;
  /** ID of the inline error element (shown on failure). */
  errorElId: string;
  /**
   * Returns the full API path (e.g. "/api/v1/dice?sides=6") after reading
   * current form values. Called inside the click handler.
   */
  buildPath(): string;
  /**
   * Renders the successful API response. Called only when res.ok is true,
   * after the result container has already been unhidden.
   */
  renderResults(json: unknown): void;
}

export function initToolRunner(options: RunToolOptions): void {
  const btn = document.querySelector<HTMLButtonElement>("[data-generate-btn]")!;
  const spinner = btn.querySelector<HTMLElement>("[data-generate-spinner]")!;
  const btnText = btn.querySelector<HTMLElement>("[data-generate-text]")!;
  const resultContainer = document.getElementById(options.resultContainerId)!;
  const errorEl = document.getElementById(options.errorElId)!;
  const curlEl = document.getElementById("api-panel-curl")!;
  const responseEl = document.getElementById("api-panel-response")!;
  const rateLimitNotice = document.querySelector<HTMLElement>("[data-rate-limit-notice]")!;
  const rateLimitOk = rateLimitNotice.querySelector<HTMLElement>("[data-rate-limit-ok]")!;
  const rateLimitText = rateLimitNotice.querySelector<HTMLElement>("[data-rate-limit-text]")!;
  const rateLimitError = rateLimitNotice.querySelector<HTMLElement>("[data-rate-limit-error]")!;
  const rateLimitErrorText = rateLimitNotice.querySelector<HTMLElement>(
    "[data-rate-limit-error-text]",
  )!;

  function setLoading(loading: boolean) {
    btn.disabled = loading;
    spinner.classList.toggle("hidden", !loading);
    btnText.textContent = loading ? btn.dataset.loadingLabel! : btn.dataset.label!;
  }

  function showError(message: string) {
    resultContainer.classList.add("hidden");
    errorEl.textContent = message;
    errorEl.classList.remove("hidden");
  }

  function setRateLimitState(state: "ok" | "error") {
    rateLimitOk.style.display = state === "ok" ? "" : "none";
    rateLimitError.style.display = state === "error" ? "inline-flex" : "none";
  }

  function updateRateLimit(
    headers: Headers,
    status: number,
    errorJson?: { detail?: string; title?: string; code?: string },
  ) {
    const remaining = headers.get("X-RateLimit-Remaining");
    const limit = headers.get("X-RateLimit-Limit");

    if (remaining === null || limit === null) return;

    rateLimitNotice.classList.remove("hidden");

    if (status === 429) {
      setRateLimitState("error");
      const msg = errorJson?.detail ?? errorJson?.title ?? `Rate limit exceeded (${limit} req/min)`;
      rateLimitErrorText.textContent = msg;
    } else {
      setRateLimitState("ok");
      rateLimitText.textContent = `${remaining} of ${limit} requests remaining in this minute`;
    }
  }

  btn.addEventListener("click", async () => {
    setLoading(true);
    errorEl.classList.add("hidden");

    const path = options.buildPath();
    curlEl.textContent = `curl ${document.documentElement.dataset.apiBaseUrl}${path}`;

    try {
      const res = await fetch(path);
      const json = await res.json();
      responseEl.textContent = JSON.stringify(json, null, 2);
      updateRateLimit(res.headers, res.status, json);

      if (!res.ok) {
        if (res.status !== 429) {
          showError(json.detail ?? json.title ?? `Error ${res.status}`);
        }
        return;
      }

      // Unhide the container before rendering: renderResults may measure or
      // initialize layout-dependent widgets (e.g. the Leaflet map on the
      // coordinates tool), which compute the wrong size in a display:none
      // container.
      resultContainer.classList.remove("hidden");
      options.renderResults(json);
    } catch {
      showError("Failed to reach the API. Please try again.");
    } finally {
      setLoading(false);
    }
  });
}
