// Applies a saved theme override before first paint, so a dark-mode user never sees a light flash.
// A separate file because the CSP allows no inline scripts. Key and values match src/lib/theme.ts.
try {
  var theme = localStorage.getItem("grower.theme");
  if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
} catch (e) {
  // Storage is blocked: follow the system.
}
