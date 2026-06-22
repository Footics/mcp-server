import "server-only";

import { createClient } from "@supabase/supabase-js";
import { createRemoteJWKSet, jwtVerify, type JWTPayload } from "jose";
import { MCP_RESOURCE, SUPABASE_ANON_KEY, SUPABASE_URL, TOKEN_VERIFY } from "@/lib/config";

export type McpIdentity = { userId: string; email: string | null };

let _supabase: ReturnType<typeof createClient> | null = null;
function supabase() {
  _supabase ??= createClient(SUPABASE_URL, SUPABASE_ANON_KEY, {
    auth: { persistSession: false, autoRefreshToken: false },
  });
  return _supabase;
}

async function verifyViaGetUser(token: string): Promise<McpIdentity | null> {
  const { data, error } = await supabase().auth.getUser(token);
  if (error || !data?.user) return null;
  return { userId: data.user.id, email: data.user.email ?? null };
}

let _jwks: ReturnType<typeof createRemoteJWKSet> | null = null;
function jwks() {
  _jwks ??= createRemoteJWKSet(new URL(`${SUPABASE_URL}/auth/v1/.well-known/jwks.json`), {
    cacheMaxAge: 600_000,
    cooldownDuration: 30_000,
  });
  return _jwks;
}

interface SupabaseClaims extends JWTPayload {
  sub: string;
  email?: string;
  role?: string;
}

async function verifyViaJwks(token: string): Promise<McpIdentity | null> {
  const { payload } = await jwtVerify(token, jwks(), {
    issuer: `${SUPABASE_URL}/auth/v1`,
    audience: ["authenticated", MCP_RESOURCE],
    clockTolerance: 5,
  });
  const claims = payload as SupabaseClaims;
  if (!claims.sub) return null;
  return { userId: claims.sub, email: claims.email ?? null };
}

export async function verifyToken(token: string | undefined): Promise<McpIdentity | null> {
  if (!token) return null;
  try {
    return TOKEN_VERIFY === "jwks" ? await verifyViaJwks(token) : await verifyViaGetUser(token);
  } catch (err) {
    console.error("[mcp-auth] token verification failed:", err);
    return null;
  }
}
