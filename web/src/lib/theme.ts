export type Theme = "system" | "light" | "dark";

/** Also read by public/theme.js, which applies the theme before first paint. */
export const THEME_KEY = "grower.theme";

/** The saved theme, or "system" when none is saved or storage is blocked. */
export function readTheme(): Theme {
  try {
    const saved = localStorage.getItem(THEME_KEY);
    return saved === "light" || saved === "dark" ? saved : "system";
  } catch {
    return "system";
  }
}

/** Saves and applies a theme. Storage can be blocked; the theme still applies for this visit. */
export function saveTheme(theme: Theme) {
  try {
    if (theme === "system") localStorage.removeItem(THEME_KEY);
    else localStorage.setItem(THEME_KEY, theme);
  } catch {
    // Not saved, but nothing else breaks.
  }
  applyTheme(theme);
}

/** Sets `data-theme` (the tokens read it) and points the browser's theme colour at the page. */
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === "system") delete root.dataset.theme;
  else root.dataset.theme = theme;

  const surface = getComputedStyle(root).getPropertyValue("--surface").trim();
  for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    // index.html has one meta per colour scheme; keep their colours for "system".
    meta.dataset.system ??= meta.content;
    meta.content = theme === "system" || !surface ? meta.dataset.system : surface;
  }
}
