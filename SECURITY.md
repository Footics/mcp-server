# Security model

Footics MCP is a remote MCP server that exposes a single Footics user's own data
to whichever AI assistant they connect. This document describes what the server
can and cannot do, and how it is hardened.

## Authentication

- Every `/mcp` request requires a valid bearer token (`MCP_REQUIRE_AUTH=true`).
  No token returns `401` with the OAuth `WWW-Authenticate` challenge and resource
  metadata (RFC 9728).
- Tokens are Supabase access tokens — issued either by the interactive OAuth 2.1
  flow ("Sign in with Footics") or as a session token for testing. They are
  verified one of two ways (`MCP_TOKEN_VERIFY`): `getuser` resolves the user
  against Supabase, `jwks` verifies the signature locally against the project's
  JWKS.
- The verified token's subject is the only identity the tools act as. There is
  no way to act on behalf of another user.

## What the server can do

| Capability | Scope |
|---|---|
| Read matches, standings, leaderboards, groups, jokers | Game data visible to the user |
| Read predictions | **Only the caller's own** — never another player's pick on an upcoming match |
| Write a prediction (`submit_prediction`) | **Only the caller's own**, and only when `MCP_ENABLE_WRITES=true` |

Writes reuse the main app's guardrails: the match must not have kicked off,
scores are bounded (0–20), and the per-phase joker quota is enforced.

## What the server cannot do

- Act as, or read the private predictions of, another user.
- Reach anything beyond the database-backed tools registered in `lib/tools.ts`.
- Run while the kill switch is off: `MCP_ENABLED=false` makes `/mcp` return `503`
  before any token check or database call.

## Hardening

- **Auth required, writes off by default.** A fresh deployment is read-only.
- **Rate limiting**: a per-user tool-call limit (`MCP_RATE_LIMIT_PER_MIN`,
  default 30), in-memory and per-instance.
- **Kill switch**: `MCP_ENABLED=false` cuts all database traffic with a single
  environment change.

## Known limitations

- **Rate limiting is per-instance.** Across several serverless instances the
  effective limit is higher. For a hard global limit, back `lib/rate-limit.ts`
  with a shared store (e.g. Upstash Redis).
- **`getuser` verification costs one network round-trip per request.** `jwks`
  avoids it but requires the Supabase project to use asymmetric signing keys.
- **The server trusts the database's row scoping.** Tools always filter by the
  authenticated user id; row-level security on the shared database is recommended
  as defense in depth.

## Reporting vulnerabilities

For non-sensitive issues, open a GitHub issue.

For sensitive vulnerabilities — anything that could let someone read or write
data they should not — email **tom@tomca.be** privately first, or open a private
security advisory on GitHub. Coordinated disclosure is appreciated; acknowledgment
within 72 hours.
