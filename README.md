# Footics MCP

A remote [MCP](https://modelcontextprotocol.io) server for [Footics](https://footics.app),
a World Cup 2026 prediction game. A user connects their AI assistant (Claude,
ChatGPT, Cursor…) to `https://mcp.footics.app/mcp`, signs in with their Footics
account, and the assistant can read their matches, standings and predictions —
and, optionally, submit them. Inference runs on the user's side; Footics only
pays for hosting.

Written in **Go** (`cmd/mcp`) with the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).
The server is a **thin authenticated client of the Footics REST API** (`/v1`):
it holds no database connection and no service-role key — every read and write
is forwarded to the API with the calling user's token, so the API stays the
single source of truth and the single write point.

## Architecture

```
AI assistant (Claude / ChatGPT)  ──MCP/HTTP──►  cmd/mcp        (/mcp)
                                                 │ internal/auth      verify user JWT (JWKS or HS256)
                                                 │ internal/tools  →  internal/apiclient  (calls /v1 with the user token)
                                                 ▼
                                          Footics API (api.footics.app) ──► Postgres
```

- **`internal/auth`** — verifies Supabase-issued user JWTs (JWKS asymmetric in
  prod, HS256 for local dev), same JWKS cache + refresh cooldown as the API.
- **`internal/apiclient`** — forwards each tool call to the Footics `/v1` API
  with the user's bearer token (no privileged credentials here).
- **`internal/tools`** — the MCP tool definitions + per-user rate limiting.

## Tools

`whoami` · `list_matches` · `get_match` · `get_my_standing` · `get_my_predictions` ·
`get_leaderboard` · `list_my_groups` · `get_joker_status` · `search` ·
`submit_prediction` *(gated by `MCP_ENABLE_WRITES`)*

## Configuration

| Variable | Default | Effect |
|---|---|---|
| `FOOTICS_API_URL` | — | base URL of the Footics REST API |
| `AUTH_JWKS_URL` | — | JWKS URL(s) for asymmetric token verification (prod) |
| `AUTH_JWT_SECRET` | — | HS256 secret (local dev alternative to JWKS) |
| `MCP_ENABLED` | `true` | `false` → `/mcp` returns 503 without verifying tokens (kill switch) |
| `MCP_REQUIRE_AUTH` | `true` | require a valid bearer token (401 otherwise) |
| `MCP_ENABLE_WRITES` | `false` | allow `submit_prediction` |
| `MCP_RATE_LIMIT_PER_MIN` | `30` | tool calls per minute per user (`0` = off) |

See [`.env.example`](./.env.example) for the full list.

## Development

```bash
cp .env.example .env    # point AUTH at a local/staging GoTrue, not production
go run ./cmd/mcp        # serves on $HTTP_ADDR (default :8080), /mcp
```

Without an `Authorization: Bearer <token>` header the endpoint returns `401`
(unless `MCP_REQUIRE_AUTH=false`). Build a static binary with
`go build ./cmd/mcp`; the container image is the multi-stage
[`Dockerfile`](./Dockerfile) (distroless).

## Deployment

Runs on the Footics VPS as a Dokploy service (`footics-mcp`), build-from-git via
[`deploy/compose.dokploy.yml`](./deploy/compose.dokploy.yml) → published at
`mcp.footics.app`. Runbook lives in the private infra repo.

## Security

Auth is required and writes are off by default. See [SECURITY.md](./SECURITY.md)
for the model and how to report a vulnerability.

## License

[MIT](./LICENSE) © 2026 Tom Cardoen
