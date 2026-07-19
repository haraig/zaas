/**
 * Inline copy-button helper for dynamically generated result rows.
 *
 * Use createInlineCopyButton(text) to create a fully wired copy button
 * that copies `text` to the clipboard. The button uses the same visual
 * style as the static CopyButton Astro component.
 */
export function createInlineCopyButton(text: string): HTMLButtonElement {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className =
    "inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium text-text-secondary dark:text-text-secondary-dark hover:text-accent transition-colors flex-shrink-0";
  btn.setAttribute("aria-label", "Copy to clipboard");
  btn.innerHTML = `
    <svg class="w-4 h-4" data-copy-icon aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
    </svg>
    <svg class="hidden w-4 h-4 text-success dark:text-success-dark" data-check-icon aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
    </svg>
    <span data-copy-label>Copy</span>`;

  btn.addEventListener("click", async () => {
    await navigator.clipboard.writeText(text);
    const copyIcon = btn.querySelector("[data-copy-icon]") as HTMLElement;
    const checkIcon = btn.querySelector("[data-check-icon]") as HTMLElement;
    const label = btn.querySelector("[data-copy-label]") as HTMLElement;
    copyIcon.classList.add("hidden");
    checkIcon.classList.remove("hidden");
    label.textContent = "Copied!";
    setTimeout(() => {
      copyIcon.classList.remove("hidden");
      checkIcon.classList.add("hidden");
      label.textContent = "Copy";
    }, 2000);
  });

  return btn;
}
