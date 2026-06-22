import { sql } from "drizzle-orm";
import { db } from "@/db";
import { ENABLE_WRITES, MCP_ENABLED, MCP_RESOURCE, RATE_LIMIT_PER_MIN, REQUIRE_AUTH, TOKEN_VERIFY } from "@/lib/config";

export const runtime = "nodejs";

export async function GET(): Promise<Response> {
  if (!MCP_ENABLED) {
    return Response.json(
      { ok: true, enabled: false, resource: MCP_RESOURCE, reason: "MCP_ENABLED=false — service coupé volontairement (maintenance), aucun appel Supabase/DB." },
      { status: 200, headers: { "Cache-Control": "no-store" } },
    );
  }

  let dbOk = false;
  let dbError: string | null = null;
  try {
    await db.execute(sql`select 1`);
    dbOk = true;
  } catch (err) {
    dbError = err instanceof Error ? err.message : String(err);
  }

  return Response.json(
    {
      ok: dbOk,
      enabled: true,
      resource: MCP_RESOURCE,
      auth: { required: REQUIRE_AUTH, tokenVerify: TOKEN_VERIFY },
      writes: ENABLE_WRITES,
      rateLimitPerMin: RATE_LIMIT_PER_MIN,
      db: { ok: dbOk, error: dbError },
    },
    { status: dbOk ? 200 : 503, headers: { "Cache-Control": "no-store" } },
  );
}
