import { createMcpHandler, withMcpAuth } from "mcp-handler";
import type { AuthInfo } from "@modelcontextprotocol/sdk/server/auth/types.js";
import { verifyToken } from "@/lib/auth";
import { ENABLE_WRITES, MCP_ENABLED, REQUIRE_AUTH, SCOPE_READ, SCOPE_WRITE, configWarnings } from "@/lib/config";
import { registerTools } from "@/lib/tools";

export const runtime = "nodejs";
export const maxDuration = 60;

for (const w of configWarnings()) console.warn("[mcp-config]", w);

const handler = createMcpHandler(
  (server) => registerTools(server),
  {},
  { basePath: "", maxDuration: 60, verboseLogs: process.env.NODE_ENV !== "production" },
);

const verify = async (_req: Request, bearerToken?: string): Promise<AuthInfo | undefined> => {
  const id = await verifyToken(bearerToken);
  if (!id) return undefined;
  const scopes = ENABLE_WRITES ? [SCOPE_READ, SCOPE_WRITE] : [SCOPE_READ];
  return {
    token: bearerToken as string,
    clientId: id.userId,
    scopes,
    extra: { userId: id.userId, email: id.email },
  };
};

const authed = withMcpAuth(handler, verify, {
  required: REQUIRE_AUTH,
  resourceMetadataPath: "/.well-known/oauth-protected-resource/mcp",
});

const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Authorization, Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Accept",
  "Access-Control-Expose-Headers": "Mcp-Session-Id, WWW-Authenticate",
};

function withCors(h: (req: Request) => Promise<Response>) {
  return async (req: Request): Promise<Response> => {
    const res = await h(req);
    const out = new Response(res.body, res);
    for (const [k, v] of Object.entries(CORS)) out.headers.set(k, v);
    return out;
  };
}

async function serviceOff(): Promise<Response> {
  return Response.json(
    {
      jsonrpc: "2.0",
      error: { code: -32000, message: "Footics MCP est temporairement hors service (maintenance). Réessaie plus tard." },
      id: null,
    },
    { status: 503, headers: { "Retry-After": "3600" } },
  );
}

const gated = (req: Request): Promise<Response> => (MCP_ENABLED ? authed(req) : serviceOff());

const GET = withCors(gated);
const POST = withCors(gated);

export async function OPTIONS(): Promise<Response> {
  return new Response(null, { status: 204, headers: CORS });
}

export { GET, POST };
