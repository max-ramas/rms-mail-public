"use client";

import React, { useEffect, useState } from "react";

import { Sun, Moon } from "lucide-react";

export function ThemeToggle() {
  const [dark, setDark] = useState(() => {
    if (typeof window !== "undefined") {
      const saved = localStorage.getItem("rms-mail_theme");
      return saved ? saved === "dark" : true;
    }
    return true;
  });

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
  }, [dark]);

  const toggle = () => {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
    localStorage.setItem("rms-mail_theme", next ? "dark" : "light");
    // Persist in cookie for server-side reading (avoids flash on reload)
    const d = new Date();
    d.setFullYear(d.getFullYear() + 1);
    document.cookie = `theme=${next ? "dark" : "light"}; path=/; expires=${d.toUTCString()}; SameSite=Lax`;
  };

  return (
    <button
      onClick={toggle}
      className="text-muted-foreground hover:text-foreground transition-colors px-2 py-1 rounded hover:bg-accent flex items-center justify-center"
      title={dark ? "Switch to light theme" : "Switch to dark theme"}
    >
      {dark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
    </button>
  );
}
