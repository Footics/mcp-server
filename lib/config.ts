export const MCP_PUBLIC_URL = (process.env.MCP_PUBLIC_URL ?? "https://mcp.footics.app").replace(/\/$/, "");

export const MCP_RESOURCE = `${MCP_PUBLIC_URL}/mcp`;

export const SUPABASE_URL = (process.env.NEXT_PUBLIC_SUPABASE_URL ?? process.env.SUPABASE_URL ?? "").replace(/\/$/, "");
export const SUPABASE_ANON_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? process.env.SUPABASE_ANON_KEY ?? "";

export const MCP_ENABLED = (process.env.MCP_ENABLED ?? "true") !== "false";

export const REQUIRE_AUTH = (process.env.MCP_REQUIRE_AUTH ?? "true") !== "false";

export const ENABLE_WRITES = (process.env.MCP_ENABLE_WRITES ?? "false") === "true";

export const TOKEN_VERIFY = (process.env.MCP_TOKEN_VERIFY ?? "getuser") as "getuser" | "jwks";

export const OAUTH_ISSUER = process.env.MCP_OAUTH_ISSUER ?? (SUPABASE_URL ? `${SUPABASE_URL}/auth/v1` : "");

export const SCOPE_READ = "footics:read";
export const SCOPE_WRITE = "footics:pronostics";

const rawRateLimit = Number(process.env.MCP_RATE_LIMIT_PER_MIN ?? "30");
export const RATE_LIMIT_PER_MIN = Number.isFinite(rawRateLimit) ? rawRateLimit : 30;

export const TEST_USER_ID = process.env.MCP_TEST_USER_ID ?? "";

export function configWarnings(): string[] {
  const w: string[] = [];
  if (!SUPABASE_URL) w.push("SUPABASE_URL/NEXT_PUBLIC_SUPABASE_URL missing — auth will fail.");
  if (!SUPABASE_ANON_KEY) w.push("SUPABASE_ANON_KEY/NEXT_PUBLIC_SUPABASE_ANON_KEY missing — auth (getuser) will fail.");
  if (!process.env.DATABASE_URL) w.push("DATABASE_URL missing — every query will fail.");
  return w;
}
