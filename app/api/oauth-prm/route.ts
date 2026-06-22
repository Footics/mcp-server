import { MCP_RESOURCE, OAUTH_ISSUER } from "@/lib/config";

export const runtime = "nodejs";

export async function GET(): Promise<Response> {
  const body = {
    resource: MCP_RESOURCE,
    authorization_servers: OAUTH_ISSUER ? [OAUTH_ISSUER] : [],
    bearer_methods_supported: ["header"],
    scopes_supported: ["openid", "email", "profile"],
    resource_name: "Footics MCP",
    resource_documentation: "https://footics.app",
  };
  return Response.json(body, {
    headers: {
      "Access-Control-Allow-Origin": "*",
      "Cache-Control": "public, max-age=3600",
    },
  });
}

export async function OPTIONS(): Promise<Response> {
  return new Response(null, {
    status: 204,
    headers: { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Methods": "GET, OPTIONS", "Access-Control-Allow-Headers": "*" },
  });
}
