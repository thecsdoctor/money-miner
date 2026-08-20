import React from "react";
import { useT } from "../i18n";

// ThemeToggle — dark is the default brand look; light via data-theme.
export function ThemeToggle() {
  const [theme, setThemeState] = React.useState<string>(
    () => localStorage.getItem("mm.theme") || "dark",
  );
  const apply = (next: string) => {
    localStorage.setItem("mm.theme", next);
    setThemeState(next);
    if (next === "light") {
      document.documentElement.dataset.theme = "light";
    } else {
      delete document.documentElement.dataset.theme;
    }
  };
  return (
    <button
      className="small"
      onClick={() => apply(theme === "light" ? "dark" : "light")}
      aria-label="toggle theme"
      title={theme === "light" ? "dark" : "light"}
    >
      {theme === "light" ? "☾" : "☀"}
    </button>
  );
}

// applyStoredTheme runs once at boot (before first paint in main.tsx).
export function applyStoredTheme() {
  const stored = localStorage.getItem("mm.theme");
  const theme =
    stored || (window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark");
  if (theme === "light") {
    document.documentElement.dataset.theme = "light";
  } else {
    delete document.documentElement.dataset.theme;
  }
}

export function LangSwitcher() {
  const { lang, setLang } = useT();
  return (
    <select
      aria-label="language"
      value={lang}
      onChange={(e) => setLang(e.target.value as "en" | "de")}
      style={{ width: "auto" }}
    >
      <option value="en">EN</option>
      <option value="de">DE</option>
    </select>
  );
}
