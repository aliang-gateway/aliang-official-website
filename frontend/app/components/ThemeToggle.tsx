"use client";

import { useEffect, useState } from "react";

type ThemeMode = "system" | "light" | "dark";

function isThemeMode(value: string | null): value is ThemeMode {
  return value === "system" || value === "light" || value === "dark";
}

function applyDocumentTheme(isDark: boolean) {
  document.documentElement.classList.toggle("dark", isDark);
}

export default function ThemeToggle() {
  const [mode, setMode] = useState<ThemeMode>("system");
  const [isDark, setIsDark] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    const stored = localStorage.getItem("theme");
    const initialMode: ThemeMode = isThemeMode(stored) ? stored : "system";
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const initialDark = initialMode === "system" ? prefersDark : initialMode === "dark";

    setMode(initialMode);
    setIsDark(initialDark);
    applyDocumentTheme(initialDark);
  }, []);

  useEffect(() => {
    if (!mounted || mode !== "system") {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const handleChange = (event: MediaQueryListEvent) => {
      setIsDark(event.matches);
      applyDocumentTheme(event.matches);
    };

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    }

    mediaQuery.addListener(handleChange);
    return () => mediaQuery.removeListener(handleChange);
  }, [mounted, mode]);

  const toggleTheme = () => {
    const nextMode: ThemeMode = mode === "system" ? "light" : mode === "light" ? "dark" : "system";
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const newDark = nextMode === "system" ? prefersDark : nextMode === "dark";

    setMode(nextMode);
    setIsDark(newDark);
    localStorage.setItem("theme", nextMode);
    applyDocumentTheme(newDark);
  };

  const label =
    mode === "system"
      ? `Follow system (${isDark ? "dark" : "light"})`
      : mode === "dark"
        ? "Dark mode"
        : "Light mode";

  if (!mounted) {
    return (
      <button
        type="button"
        className="theme-toggle"
        aria-label="Toggle theme"
        disabled
      >
        <span className="theme-toggle-icon" />
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={toggleTheme}
      className="theme-toggle"
      title={label}
      aria-label={`Theme mode: ${label}. Click to switch.`}
    >
      {mode === "system" ? (
        <svg
          className="theme-toggle-icon"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9.75 17L8.5 20.5M14.25 17l1.25 3.5M4 5.75h16a1 1 0 011 1V15a1 1 0 01-1 1H4a1 1 0 01-1-1V6.75a1 1 0 011-1z"
          />
        </svg>
      ) : isDark ? (
        <svg
          className="theme-toggle-icon"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
          />
        </svg>
      ) : (
        <svg
          className="theme-toggle-icon"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
          />
        </svg>
      )}
    </button>
  );
}
