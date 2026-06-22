"use client";

import { useState } from "react";

export function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // clipboard API unavailable; the command text stays selectable
    }
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={copied ? "Copié" : "Copier la commande"}
      style={{
        flexShrink: 0,
        border: "none",
        borderRadius: 9,
        padding: "6px 12px",
        fontSize: 12,
        fontWeight: 700,
        fontFamily: "inherit",
        cursor: "pointer",
        background: copied ? "var(--f-primary-soft)" : "var(--f-ink)",
        color: copied ? "var(--f-primary-deep)" : "var(--f-paper)",
        transition: "background .15s ease, color .15s ease",
      }}
    >
      {copied ? "Copié ✓" : "Copier"}
    </button>
  );
}
