# Footics MCP — Go rewrite (client of `/v1`)

Go port of the Footics MCP server. It is a **thin HTTP client of `footics-api`
`/v1`**: it verifies the Supabase JWT locally, then relays the caller's Bearer to
the API — **no database access**. This is the staging deliverable for PRD 15
(`docs/prd/15-mcp-go.md`); it lives in `go/` alongside the untouched Next.js/TS
server at the repo root.

> **The TypeScript server stays.** Nothing at the repo root is removed or moved.
> The live server (`mcp.footics.app` on Vercel) is unaffected. Whether/when Go
> replaces TS is **Tom's call** — see [Repo structure](#repo-structure).

## What it does

- Verifies the Supabase JWT **locally**, double-mode (JWKS ES256/RS256 or HS256),
  same logic as `footics-api/internal/auth`.
- Serves RFC 9728 **protected-resource metadata** on both well-known paths + the
  `WWW-Authenticate` challenge on 401 → OAuth discovery still points at Supabase.
- Exposes the **10 frozen tools** over Streamable HTTP (stateless). Each read is
  one `/v1` call with the Bearer relayed; the output shape is byte-identical to
  the TS server (`lib/tools.ts`). `whoami` is served from the JWT alone.

## Quickstart

```bash
cd go
cp .env.example .env            # point FOOTICS_API_URL at the staging API
set -a && . ./.env && set +a
go run ./cmd/mcp                 # listens on HTTP_ADDR (default :8080)
```

Gate (what CI runs):

```bash
go build ./... && go vet ./... && gofmt -l .   # last must print nothing
go test ./...
```

The integration test (`internal/tools/integration_test.go`) stands up the real
stack in front of a running staging API (`http://127.0.0.1:8099`), mints an alice
Bearer, and asserts the frozen shape of every read tool. It **skips** cleanly
when the API is unreachable.

## Environment

| Var | Required | Default | Notes |
|---|---|---|---|
| `FOOTICS_API_URL` | ✅ | — | Base URL of `footics-api` (`/v1` appended). No `DATABASE_URL` exists. |
| `AUTH_JWKS_URL` \| `AUTH_JWT_SECRET` | ✅ (exactly one) | — | JWKS (prod) or HS256 (staging). Mutually exclusive. |
| `AUTH_JWT_AUD` | | `authenticated` | Expected `aud` claim. |
| `MCP_PUBLIC_URL` | | `https://mcp.footics.app` | `Resource` = this + `/mcp`. |
| `MCP_OAUTH_ISSUER` | | — | `authorization_servers[0]` in the metadata (Supabase). |
| `HTTP_ADDR` | | `:8080` | |
| `MCP_ENABLED` | | `true` | `false` → 503 kill switch. |
| `MCP_REQUIRE_AUTH` | | `true` | `false` → authless test mode (`MCP_TEST_USER_ID`). |
| `MCP_ENABLE_WRITES` | | `false` | See [submit_prediction](#submit_prediction-awaits-m4). |
| `MCP_RATE_LIMIT_PER_MIN` | | `30` | Per-user; 0 = off. Global on this single process. |

## Tools → `/v1` endpoints

| Tool | Endpoint | Notes |
|---|---|---|
| `whoami` | — (JWT) | `{userId, email}` from the token. |
| `list_matches` | `GET /v1/matches?competition=&status=&limit=` | flat array, projected to the frozen `MatchJson`. |
| `get_match` | `GET /v1/matches/{id}` | + `events[]` (`[]` when scheduled); 404 → "Match introuvable." |
| `get_my_standing` | `GET /v1/me?competition=` | projects `{userId,username,competition,points,exact,rank,rankOf}`; 404 → "Profil introuvable." |
| `get_my_predictions` | `GET /v1/predictions?competition=&when=&limit=` | drops `winnerTeamCode`; `result`/`points` present-null. |
| `get_leaderboard` | `GET /v1/leaderboard?competition=&group=&limit=` | projects `rows`. |
| `list_my_groups` | `GET /v1/groups?competition=` | projects `groups` (`owner` bool). |
| `get_joker_status` | `GET /v1/jokers?competition=` | passthrough. |
| `search` | `GET /v1/search?q=` | passthrough (`{matches,groups,users}`). |
| `submit_prediction` | `POST /v1/predictions` | **stub — awaits M4.** |

Projection notes (to stay byte-identical to `lib/tools.ts`):
- **`kickoffAt`** is normalised to `Date.toISOString()` form (always `.000Z`) —
  `/v1/matches` already emits millis, `/v1/predictions` and `/v1/search` do not.
- **`venue`** empty string → `null` (the TS `venue ?? null` for an absent venue).
- **`score`** = the regulation `Score` for finished, `LiveScore` for live, else `null`.
- Input schemas are **fixed explicitly** (`internal/tools/schemas.go`) — enums,
  bounds, defaults and FR descriptions — not inferred from Go structs, so
  `tools/list` matches the zod-derived TS schemas.

### `submit_prediction` awaits M4

The write path (`footics-api POST /v1/predictions`) ships with **M4** and is not
delivered yet. The tool is registered (prod runs writes ON) but is a **stub**:
with `MCP_ENABLE_WRITES=false` it returns *"écriture pas encore disponible sur
cette instance"*; with writes on it returns a *"arrivera avec l'API /v1 POST
(M4)"* error. Wiring the real call is a one-function change once M4 lands.

## Layout

```
go/
  cmd/mcp/            server wiring (mux, PRM, auth middleware, CORS, kill switch, health)
  internal/config/    env parsing (no DATABASE_URL)
  internal/auth/      JWT double-mode verifier → go-sdk auth.TokenVerifier
  internal/apiclient/ thin /v1 HTTP client (relays the Bearer)
  internal/tools/     10 tools: registration, gate (auth+rate-limit), projections, schemas
```

Uses the official SDK `github.com/modelcontextprotocol/go-sdk` (v1.6.1):
`mcp.NewStreamableHTTPHandler` (stateless), `mcp.AddTool`, `auth.RequireBearerToken`,
`auth.ProtectedResourceMetadataHandler`. Routing is stdlib `net/http.ServeMux`
(five static routes — chi would buy nothing here).

## Repo structure

Delivered under `go/` so the branch is **purely additive**: not a single TS file
at the repo root is moved or deleted, and the live Vercel server is untouched.
PRD 15 §9 envisions an eventual in-place rewrite (TS → history + a `v0-nextjs`
tag); promoting `go/` to the root and archiving the TS is a **separate decision
for Tom**, out of scope for this staging deliverable.

Not included here (land with the cutover, N4): Dockerfile / GHCR image, Go CI
workflow, tunnel + compose service. See PRD 15 §10–11.
