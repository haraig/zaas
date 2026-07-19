// Theme initialisation script - loaded synchronously to prevent flash of
// unstyled content (FOUC). Reads the persisted theme preference from
// localStorage and applies the "dark" class to <html> before the page paints.
// Served as a static asset so no inline scripts are needed in HTML pages,
// which allows a strict Content-Security-Policy without 'unsafe-inline'.
(function () {
  var stored = localStorage.getItem("theme");
  var prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  var isDark = stored === "dark" || (!stored && prefersDark);
  document.documentElement.classList.toggle("dark", isDark);
})();
