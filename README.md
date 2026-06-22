# Footics MCP

A remote [MCP](https://modelcontextprotocol.io) server for [Footics](https://footics.app),
a World Cup 2026 prediction game. A user connects their AI assistant (Claude,
ChatGPT, Cursor…) to `https://mcp.footics.app/mcp`, signs in with their Footics
account, and the assistant can read their matches, standings and predictions —
and, optionally, submit them. Inference runs on the user's side; Footics only
pays for hosting.

Separate Vercel project from the main site, **same Supabase database**. Built with
Next.js 16 (App Router) and [`mcp-handler`](https://github.com/vercel/mcp-handler).

## Architecture

```
AI assistant (Claude / ChatGPT)  ──MCP/HTTP──►  app/[transport]/route.ts  (/mcp)
                                                 │ withMcpAuth → lib/auth      (verify Supabase JWT)
                                                 │ lib/tools  → lib/queries     (reads)
                                                 │            → lib/predictions  (writes, gated)
                                                 ▼
                                          Supabase Postgres (shared with the site)
```

- **`lib/queries.ts`** — lean reads that return raw JSON for the model, not the UI view-models.
- **`lib/predictions.ts`** — writes with the same guardrails as the main app. Off by default (`MCP_ENABLE_WRITES=false`).
- **`db/schema.ts`**, **`db/index.ts`**, **`lib/jokers.ts`** — mirror the main app's schema and rules so column contracts never drift.

## Tools

`whoami` · `list_matches` · `get_match` · `get_my_standing` · `get_my_predictions` ·
`get_leaderboard` · `list_my_groups` · `get_joker_status` · `search` ·
`submit_prediction` *(gated)*

## Configuration

| Variable | Default | Effect |
|---|---|---|
| `MCP_ENABLED` | `true` | `false` → `/mcp` returns 503 with no Supabase/DB call (kill switch) |
| `MCP_REQUIRE_AUTH` | `true` | require a valid bearer token (401 otherwise) |
| `MCP_ENABLE_WRITES` | `false` | allow `submit_prediction` |
| `MCP_TOKEN_VERIFY` | `getuser` | `getuser` (robust) or `jwks` (local, asymmetric) |
| `MCP_RATE_LIMIT_PER_MIN` | `30` | tool calls per minute per user (`0` = off) |

See [`.env.example`](./.env.example) for the full list, including the Supabase
connection variables.

## Development

```bash
pnpm install
cp .env.example .env.local   # fill in — point at a staging project, not production
pnpm dev                     # http://localhost:3000/mcp
pnpm token                   # mint a test access token + ready-to-paste MCP Inspector commands
```

`pnpm token` prints an MCP Inspector command: run `tools/list`, then `whoami`,
then any tool. Without an `Authorization` header the endpoint returns `401`.

## Security

Auth is required and writes are off by default. See [SECURITY.md](./SECURITY.md)
for the model and how to report a vulnerability.

## License

[MIT](./LICENSE) © 2026 Tom Cardoen
