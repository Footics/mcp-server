import { createHash, randomBytes } from "node:crypto";
import { writeFileSync } from "node:fs";

const BASE = (process.env.NEXT_PUBLIC_SUPABASE_URL ?? "").replace(/\/$/, "");
const ISSUER = `${BASE}/auth/v1`;
const APIKEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "";
const TEST_EMAIL = process.env.TEST_EMAIL ?? "";
const TEST_PASSWORD = process.env.TEST_PASSWORD ?? "";
const MCP_URL = `${(process.env.MCP_PUBLIC_URL ?? "https://mcp.footics.app").replace(/\/$/, "")}/mcp`;
const REDIRECT_URI = "http://localhost:8976/cb";
if (!BASE || !APIKEY || !TEST_EMAIL || !TEST_PASSWORD) {
  console.error("Missing variables (.env.local): NEXT_PUBLIC_SUPABASE_URL, NEXT_PUBLIC_SUPABASE_ANON_KEY, TEST_EMAIL, TEST_PASSWORD");
  process.exit(1);
}

const b64url = (buf) => Buffer.from(buf).toString("base64url");
const decodeJwt = (tok) => {
  const [h, p] = tok.split(".");
  return { header: JSON.parse(Buffer.from(h, "base64url")), claims: JSON.parse(Buffer.from(p, "base64url")) };
};
const step = (n, msg) => console.log(`\n[${n}] ${msg}`);

step(1, "Discovery (RFC 8414)");
const disco = await (await fetch(`${BASE}/.well-known/oauth-authorization-server/auth/v1`)).json();
console.log("issuer:", disco.issuer, "| DCR:", !!disco.registration_endpoint, "| scopes:", disco.scopes_supported.join(" "));

step(2, "Dynamic client registration");
const regRes = await fetch(disco.registration_endpoint, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    client_name: "Footics MCP e2e probe",
    redirect_uris: [REDIRECT_URI],
    token_endpoint_auth_method: "none",
    grant_types: ["authorization_code", "refresh_token"],
    response_types: ["code"],
  }),
});
const reg = await regRes.json();
if (!regRes.ok) throw new Error(`DCR failed ${regRes.status}: ${JSON.stringify(reg)}`);
console.log("client_id:", reg.client_id, "| type:", reg.client_type ?? "?");

step(3, "GET /oauth/authorize (PKCE S256, standard scope)");
const verifier = b64url(randomBytes(48));
const challenge = b64url(createHash("sha256").update(verifier).digest());
const state = b64url(randomBytes(12));
const authorizeUrl = new URL(`${ISSUER}/oauth/authorize`);
authorizeUrl.search = new URLSearchParams({
  client_id: reg.client_id,
  redirect_uri: REDIRECT_URI,
  response_type: "code",
  code_challenge: challenge,
  code_challenge_method: "S256",
  scope: "openid email profile",
  state,
}).toString();
const authRes = await fetch(authorizeUrl, { redirect: "manual" });
const location = authRes.headers.get("location");
console.log("status:", authRes.status, "→", location);
if (!location) throw new Error(`no redirect: ${await authRes.text()}`);
const authorizationId = new URL(location).searchParams.get("authorization_id");
if (!authorizationId) throw new Error("authorization_id missing from redirect");
console.log("authorization_id:", authorizationId);

step(4, "Log in test account (password grant)");
const loginRes = await fetch(`${ISSUER}/token?grant_type=password`, {
  method: "POST",
  headers: { "Content-Type": "application/json", apikey: APIKEY },
  body: JSON.stringify({ email: TEST_EMAIL, password: TEST_PASSWORD }),
});
const session = await loginRes.json();
if (!loginRes.ok) throw new Error(`login failed: ${JSON.stringify(session)}`);
console.log("session OK (aud:", decodeJwt(session.access_token).claims.aud + ")");

step(5, "GET authorization details (as the consent page does)");
const detRes = await fetch(`${ISSUER}/oauth/authorizations/${authorizationId}`, {
  headers: { apikey: APIKEY, Authorization: `Bearer ${session.access_token}` },
});
const details = await detRes.json();
console.log(detRes.status, JSON.stringify({ client: details.client?.name, scope: details.scope, redirect_uri: details.redirect_uri, user: details.user?.email }));

step(6, "POST consent approve");
const appRes = await fetch(`${ISSUER}/oauth/authorizations/${authorizationId}/consent`, {
  method: "POST",
  headers: { "Content-Type": "application/json", apikey: APIKEY, Authorization: `Bearer ${session.access_token}` },
  body: JSON.stringify({ action: "approve" }),
});
const approved = await appRes.json();
if (!appRes.ok) throw new Error(`approve failed ${appRes.status}: ${JSON.stringify(approved)}`);
const cbUrl = new URL(approved.redirect_url);
const code = cbUrl.searchParams.get("code");
console.log("redirect_url → code:", code?.slice(0, 8) + "…", "| state ok:", cbUrl.searchParams.get("state") === state);

step(7, "POST /oauth/token (authorization_code + PKCE)");
const tokRes = await fetch(disco.token_endpoint, {
  method: "POST",
  headers: { "Content-Type": "application/x-www-form-urlencoded" },
  body: new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: REDIRECT_URI,
    client_id: reg.client_id,
    code_verifier: verifier,
  }),
});
const tokens = await tokRes.json();
if (!tokRes.ok) throw new Error(`token exchange failed ${tokRes.status}: ${JSON.stringify(tokens)}`);
writeFileSync("/tmp/sb-oauth-token.txt", tokens.access_token);
writeFileSync("/tmp/sb-oauth-refresh.txt", tokens.refresh_token ?? "");
const { header, claims } = decodeJwt(tokens.access_token);
console.log("alg:", header.alg, "| expires_in:", tokens.expires_in, "| refresh:", !!tokens.refresh_token);
console.log("CLAIMS:", JSON.stringify(claims, null, 1));

step(8, "GET /auth/v1/user with the OAuth token");
const guRes = await fetch(`${ISSUER}/user`, { headers: { apikey: APIKEY, Authorization: `Bearer ${tokens.access_token}` } });
const gu = await guRes.json();
console.log(guRes.status, guRes.ok ? `user: ${gu.email}` : JSON.stringify(gu));

step(9, `tools/call whoami on ${MCP_URL} with the OAuth token`);
const mcpRes = await fetch(MCP_URL, {
  method: "POST",
  headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream", Authorization: `Bearer ${tokens.access_token}` },
  body: JSON.stringify({ jsonrpc: "2.0", method: "tools/call", id: 1, params: { name: "whoami", arguments: {} } }),
});
console.log("MCP:", mcpRes.status, (await mcpRes.text()).slice(0, 250).replace(/\n/g, " "));

step(10, "grant_type=refresh_token");
const refRes = await fetch(disco.token_endpoint, {
  method: "POST",
  headers: { "Content-Type": "application/x-www-form-urlencoded" },
  body: new URLSearchParams({ grant_type: "refresh_token", refresh_token: tokens.refresh_token, client_id: reg.client_id }),
});
const refreshed = await refRes.json();
console.log(refRes.status, refRes.ok ? `new token OK (expires_in ${refreshed.expires_in})` : JSON.stringify(refreshed));

step(11, "authorize with scope=footics:read (custom-scope behavior)");
const u2 = new URL(`${ISSUER}/oauth/authorize`);
u2.search = new URLSearchParams({
  client_id: reg.client_id,
  redirect_uri: REDIRECT_URI,
  response_type: "code",
  code_challenge: challenge,
  code_challenge_method: "S256",
  scope: "footics:read",
  state,
}).toString();
const a2 = await fetch(u2, { redirect: "manual" });
const loc2 = a2.headers.get("location") ?? "";
console.log("status:", a2.status, "→", loc2.slice(0, 140), loc2.includes("error") ? "(ERROR → PRM must advertise standard scopes)" : "");

console.log("\n✓ E2E complete");
