import type { CSSProperties } from "react";

export function FooticsLogo({ size = 24, color, mono = false }: { size?: number; color?: string; mono?: boolean }) {
  const c = color || "var(--f-ink)";
  const accent = mono ? c : "var(--f-primary)";
  const display: CSSProperties = { fontFamily: "var(--font-bricolage), ui-sans-serif", fontWeight: 800, fontSize: size, letterSpacing: 0 };
  return (
    <span
      role="img"
      aria-label="Footics"
      style={{ display: "inline-flex", alignItems: "baseline", gap: 0, color: c, lineHeight: 1, whiteSpace: "nowrap" }}
    >
      <span style={display}>Foo</span>
      <span style={{ ...display, position: "relative" }}>
        t
        <span style={{ position: "relative", display: "inline-block" }}>
          ı
          <span
            style={{
              position: "absolute",
              left: "50%",
              top: -size * 0.18,
              transform: "translateX(-50%)",
              width: size * 0.3,
              height: size * 0.3,
              borderRadius: "50%",
              background: accent,
              boxShadow: `0 0 0 ${size * 0.04}px ${c === "var(--f-ink)" ? "var(--f-bg)" : "transparent"}`,
            }}
          />
        </span>
        cs
      </span>
    </span>
  );
}
