import type { CSSProperties, ReactNode } from "react";
import { CopyButton } from "@/components/copy-button";
import { FooticsLogo } from "@/components/footics-logo";
import { MCP_RESOURCE } from "@/lib/config";

const display: CSSProperties = { fontFamily: "var(--font-bricolage), ui-sans-serif", letterSpacing: "-0.03em" };
const card: CSSProperties = { background: "var(--f-surface)", borderRadius: 22, padding: "24px 26px", boxShadow: "var(--f-shadow-card)" };
const h2Style: CSSProperties = { ...display, fontSize: 22, fontWeight: 800, margin: "0 0 4px" };
const stepText: CSSProperties = { fontSize: 14.5, color: "var(--f-ink-2)", lineHeight: 1.6 };

function Code({ children }: { children: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, background: "var(--f-ink)", borderRadius: 12, padding: "11px 14px", margin: "10px 0" }}>
      <pre style={{ margin: 0, overflowX: "auto", flex: 1 }}>
        <code style={{ color: "var(--f-paper)", fontSize: 13, whiteSpace: "pre" }}>{children}</code>
      </pre>
      <CopyButton text={children} />
    </div>
  );
}

function Badge({ children, tone = "soft" }: { children: ReactNode; tone?: "soft" | "ink" }) {
  return (
    <span
      style={{
        fontSize: 11,
        fontWeight: 700,
        padding: "3px 9px",
        borderRadius: 999,
        background: tone === "ink" ? "var(--f-ink)" : "var(--f-primary-soft)",
        color: tone === "ink" ? "var(--f-paper)" : "var(--f-primary-deep)",
        whiteSpace: "nowrap",
      }}
    >
      {children}
    </span>
  );
}

function ClientCard({ name, badge, children }: { name: string; badge?: string; children: ReactNode }) {
  return (
    <section style={card}>
      <div style={{ display: "flex", alignItems: "baseline", gap: 10, flexWrap: "wrap" }}>
        <h2 style={h2Style}>{name}</h2>
        {badge && <Badge>{badge}</Badge>}
      </div>
      <div style={stepText}>{children}</div>
    </section>
  );
}

const CURSOR_JSON = `{
  "mcpServers": {
    "footics": { "url": "${MCP_RESOURCE}" }
  }
}`;

export default function Home() {
  return (
    <main style={{ maxWidth: 760, margin: "0 auto", padding: "44px 20px 70px" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 44 }}>
        <a href="https://footics.app" style={{ display: "flex", alignItems: "center", gap: 8, textDecoration: "none" }}>
          <FooticsLogo size={22} />
          <Badge tone="ink">MCP</Badge>
        </a>
        <a href="https://footics.app" style={{ fontSize: 13, fontWeight: 600, color: "var(--f-ink-3)" }}>
          footics.app →
        </a>
      </header>

      <h1 style={{ ...display, fontSize: 44, fontWeight: 800, lineHeight: 1.05, margin: "0 0 14px" }}>
        Parle à tes pronos<span style={{ color: "var(--f-primary)" }}>.</span>
      </h1>
      <p style={{ fontSize: 16.5, color: "var(--f-ink-2)", lineHeight: 1.6, margin: "0 0 8px", maxWidth: 620 }}>
        Connecte ton assistant IA à ton compte Footics : consulte ton classement, suis les matchs de la Coupe du monde
        et pose tes pronos, directement dans la conversation.
      </p>
      <p style={{ fontSize: 13.5, color: "var(--f-ink-3)", margin: "0 0 28px" }}>
        Connexion « Sign in with Footics » · tes pronos restent les tiens · gratuit, comme le reste.
      </p>

      <section style={{ ...card, marginBottom: 14 }}>
        <div style={{ fontSize: 11.5, fontWeight: 700, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--f-ink-3)", marginBottom: 2 }}>
          L&apos;adresse à connaître
        </div>
        <Code>{MCP_RESOURCE}</Code>
        <p style={{ ...stepText, margin: 0 }}>
          C&apos;est tout ce dont ton assistant a besoin. Colle-la dans l&apos;outil de ton choix ci-dessous, connecte-toi avec ton
          compte Footics quand il te le demande, et c&apos;est parti.
        </p>
      </section>

      <div style={{ display: "grid", gap: 14 }}>
        <ClientCard name="Claude" badge="claude.ai · desktop · mobile">
          <ol style={{ margin: "8px 0 0", paddingLeft: 20, display: "grid", gap: 4 }}>
            <li>
              <strong>Réglages → Connecteurs → Ajouter un connecteur personnalisé</strong>
            </li>
            <li>
              Colle l&apos;URL : <code style={{ background: "var(--f-surface-2)", padding: "1px 6px", borderRadius: 6, fontSize: 13 }}>{MCP_RESOURCE}</code>
            </li>
            <li>
              Clique « Se connecter » → identifie-toi sur footics.app → <strong>Autoriser</strong>.
            </li>
          </ol>
          <p style={{ margin: "10px 0 0", fontSize: 12.5, color: "var(--f-ink-3)" }}>
            Connecteurs personnalisés disponibles sur les plans Claude Pro, Max, Team et Enterprise.
          </p>
        </ClientCard>

        <ClientCard name="Claude Code" badge="terminal">
          <Code>{`claude mcp add --transport http footics ${MCP_RESOURCE}`}</Code>
          <p style={{ margin: 0 }}>
            Puis dans une session : tape <code style={{ background: "var(--f-surface-2)", padding: "1px 6px", borderRadius: 6, fontSize: 13 }}>/mcp</code>,
            choisis <strong>footics</strong> → <strong>Authenticate</strong>, et connecte-toi dans le navigateur.
          </p>
        </ClientCard>

        <ClientCard name="Codex" badge="OpenAI · terminal">
          <Code>{`codex mcp add footics --url ${MCP_RESOURCE}`}</Code>
          <Code>{`codex mcp login footics`}</Code>
        </ClientCard>

        <ClientCard name="ChatGPT" badge="Plus · Pro · Team">
          <ol style={{ margin: "8px 0 0", paddingLeft: 20, display: "grid", gap: 4 }}>
            <li>
              <strong>Paramètres → Connecteurs → Paramètres avancés</strong> : active le <strong>mode développeur</strong>
            </li>
            <li>
              <strong>Ajouter un connecteur personnalisé</strong> → colle l&apos;URL ci-dessus
            </li>
            <li>Connecte-toi avec ton compte Footics, puis active le connecteur dans ta conversation.</li>
          </ol>
        </ClientCard>

        <ClientCard name="Cursor" badge="éditeur">
          <p style={{ margin: "8px 0 0" }}>
            Dans <code style={{ background: "var(--f-surface-2)", padding: "1px 6px", borderRadius: 6, fontSize: 13 }}>.cursor/mcp.json</code> (projet) ou{" "}
            <code style={{ background: "var(--f-surface-2)", padding: "1px 6px", borderRadius: 6, fontSize: 13 }}>~/.cursor/mcp.json</code> (global) :
          </p>
          <Code>{CURSOR_JSON}</Code>
        </ClientCard>

        <ClientCard name="Autre client MCP">
          <p style={{ margin: "8px 0 0" }}>
            Tout client qui parle <strong>MCP Streamable HTTP + OAuth</strong> fonctionne : donne-lui l&apos;URL{" "}
            <code style={{ background: "var(--f-surface-2)", padding: "1px 6px", borderRadius: 6, fontSize: 13 }}>{MCP_RESOURCE}</code> — la découverte,
            l&apos;enregistrement et la connexion sont automatiques (RFC 9728, PKCE, refresh).
          </p>
        </ClientCard>
      </div>

      <section style={{ ...card, marginTop: 14, background: "var(--f-primary-soft)" }}>
        <h2 style={{ ...h2Style, color: "var(--f-primary-deep)" }}>Ensuite, demande-lui…</h2>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 10 }}>
          {["Quel est mon classement ?", "Les matchs de ce soir ?", "Pose 2-1 sur le match de la France", "Le top 5 de mon groupe ?", "Il me reste combien de jokers ?"].map((q) => (
            <span key={q} style={{ background: "var(--f-surface)", borderRadius: 999, padding: "7px 14px", fontSize: 13.5, fontWeight: 600, color: "var(--f-ink-2)" }}>
              « {q} »
            </span>
          ))}
        </div>
      </section>

      <section style={{ ...card, marginTop: 14 }}>
        <h2 style={h2Style}>Sécurité, en deux mots</h2>
        <ul style={{ ...stepText, margin: "8px 0 0", paddingLeft: 20, display: "grid", gap: 4 }}>
          <li>
            Tu te connectes <strong>sur footics.app</strong> (OAuth 2.1) — ton mot de passe n&apos;est jamais partagé avec l&apos;assistant.
          </li>
          <li>
            L&apos;IA ne peut écrire qu&apos;une seule chose : <strong>tes propres pronos</strong>, avec les mêmes règles que l&apos;app
            (verrouillage au coup d&apos;envoi, scores 0-20, quota de jokers) — et ton assistant te demande confirmation avant de poser.
          </li>
          <li>Tu peux refuser ou déconnecter le connecteur à tout moment, depuis ton assistant.</li>
        </ul>
      </section>

      <footer style={{ textAlign: "center", marginTop: 36, fontSize: 12.5, color: "var(--f-ink-3)", fontWeight: 500 }}>
        Outil officiel de <a href="https://footics.app">footics.app</a> · <a href="https://status.footics.app">état du service</a>
        <div style={{ marginTop: 6 }}>Gratuit · sans pub · sans paris</div>
      </footer>
    </main>
  );
}
