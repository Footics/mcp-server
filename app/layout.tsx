import type { ReactNode } from "react";
import { Bricolage_Grotesque } from "next/font/google";

const bricolage = Bricolage_Grotesque({ subsets: ["latin"], weight: ["700", "800"], variable: "--font-bricolage" });

export const metadata = {
  metadataBase: new URL("https://mcp.footics.app"),
  title: "Footics MCP — connecte ton assistant IA à tes pronos",
  description:
    "Branche Claude, ChatGPT, Codex ou Cursor sur ton compte Footics : classement, pronos et matchs de la Coupe du monde, directement dans la conversation. Lecture seule, connexion sécurisée.",
  openGraph: {
    title: "Footics MCP — parle à tes pronos",
    description: "Connecte ton assistant IA à ton compte Footics en 2 minutes.",
    url: "https://mcp.footics.app",
    siteName: "Footics MCP",
    locale: "fr_FR",
    type: "website",
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="fr" className={bricolage.variable}>
      <body
        style={{
          margin: 0,
          fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
          background: "var(--f-bg)",
          color: "var(--f-ink)",
          WebkitFontSmoothing: "antialiased",
        }}
      >
        <style>{`
          :root {
            --f-bg: #f2eee3;
            --f-paper: #faf6ec;
            --f-surface: #ffffff;
            --f-surface-2: #f7f2e6;
            --f-ink: #0e1316;
            --f-ink-2: #353a40;
            --f-ink-3: #62686e;
            --f-primary: #15c26b;
            --f-primary-deep: #0c7a41;
            --f-primary-soft: #d6f4e3;
            --f-accent: #ff5a1f;
            --f-border: rgba(14, 19, 22, .08);
            --f-shadow-card: 0 1px 0 rgba(14,19,22,.04), 0 4px 16px rgba(14,19,22,.07);
          }
          a { color: var(--f-primary-deep); text-decoration: none; }
          a:hover { text-decoration: underline; }
          code, pre { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
          @media (max-width: 560px) { h1 { font-size: 34px !important; } }
        `}</style>
        {children}
      </body>
    </html>
  );
}
