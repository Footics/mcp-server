// Package tools registers the Footics MCP tools on a go-sdk server. Each read
// tool is one authenticated /v1 call (Bearer relayed) projected into the frozen
// output shape (shapes.go). whoami is served from the JWT alone; submit_prediction
// forwards to footics-api POST /v1/predictions (the single write point).
//
// Output envelope (frozen): success = one text block of pretty-printed JSON
// (2-space indent, HTML-escaping OFF, matching JSON.stringify(data,null,2));
// error = one text block of {"error":"…"} with IsError set.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/footics/mcp-server/internal/apiclient"
	"github.com/footics/mcp-server/internal/auth"
)

// Deps are the tool layer's collaborators.
type Deps struct {
	API             *apiclient.Client
	EnableWrites    bool
	TestUserID      string // identity fallback when MCP_REQUIRE_AUTH=false (no TokenInfo)
	RateLimitPerMin int
}

type toolServer struct {
	api          *apiclient.Client
	enableWrites bool
	testUserID   string
	rl           *rateLimiter
	rlPerMin     int
}

// Register adds the 10 tools (9 reads + submit_prediction) to server.
func Register(server *mcp.Server, d Deps) {
	s := &toolServer{
		api:          d.API,
		enableWrites: d.EnableWrites,
		testUserID:   d.TestUserID,
		rl:           newRateLimiter(d.RateLimitPerMin),
		rlPerMin:     d.RateLimitPerMin,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Title:       "Qui suis-je",
		Description: "Renvoie l'identité Footics de l'utilisateur connecté (id, email). Utile pour vérifier la connexion.",
		InputSchema: schemaWhoami(),
	}, s.whoami)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_matches",
		Title:       "Lister les matchs",
		Description: "Liste les matchs d'une compétition (avec mon prono, le statut, le score live/final). Filtrable par statut. Pour voir le calendrier, les matchs à venir, en cours ou terminés.",
		InputSchema: schemaListMatches(),
	}, s.listMatches)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_match",
		Title:       "Détail d'un match",
		Description: "Renvoie un match par son id : équipes, coup d'envoi, statut, score, timeline des buts/cartons (si commencé) et mon prono.",
		InputSchema: schemaGetMatch(),
	}, s.getMatch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_my_standing",
		Title:       "Mon classement",
		Description: "Mes points, scores exacts, mon rang et le nombre total de joueurs pour une compétition.",
		InputSchema: schemaComp(),
	}, s.getMyStanding)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_my_predictions",
		Title:       "Mes pronos",
		Description: "Mes pronostics pour une compétition. `when` = upcoming (à venir), past (terminés) ou all (tous, défaut).",
		InputSchema: schemaPredictions(),
	}, s.getMyPredictions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_leaderboard",
		Title:       "Classement",
		Description: "Classement général d'une compétition, ou d'un de mes groupes (via groupId). Trié par points puis scores exacts.",
		InputSchema: schemaLeaderboard(),
	}, s.getLeaderboard)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_my_groups",
		Title:       "Mes groupes",
		Description: "Les groupes dont je suis membre, avec mon rang et mes points dans chacun pour la compétition donnée.",
		InputSchema: schemaComp(),
	}, s.listMyGroups)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_joker_status",
		Title:       "Mes jokers",
		Description: "Mon stock de jokers par bucket (poules, 8es, …) pour une compétition : utilisés / quota / restants. Le joker double les points d'un match.",
		InputSchema: schemaComp(),
	}, s.getJokerStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search",
		Title:       "Rechercher",
		Description: "Recherche transverse : matchs (par équipe/poule), mes groupes (par nom), joueurs (par pseudo).",
		InputSchema: schemaSearch(),
	}, s.search)

	// submit_prediction always appears; it forwards to footics-api
	// POST /v1/predictions and is gated at call time by MCP_ENABLE_WRITES.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "submit_prediction",
		Title:       "Poser un prono",
		Description: "Pose ou modifie MON pronostic sur un match (scores 0-20, joker optionnel). Refusé si le coup d'envoi est passé ou si je n'ai plus de joker pour ce bucket. Sur un match à élimination directe : le prono porte sur le score à la fin du temps réglementaire (90'), et si tu prédis un NUL, précise winnerTeamCode (l'équipe qui se qualifie) pour le +1 bonus. Confirme toujours avec l'utilisateur avant d'écrire.",
		InputSchema: schemaSubmit(),
	}, s.submitPrediction)
}

/* ── identity + gate (auth + rate-limit) ───────────────────────────────────── */

type identity struct {
	userID string
	token  string // the caller's Bearer, relayed to footics-api
	email  *string
}

// gate resolves the caller's identity (from the verified JWT, or the TestUserID
// fallback in authless mode) and applies the per-user rate limit. A non-nil
// result is the error to return from the tool.
func (s *toolServer) gate(ctx context.Context) (identity, *mcp.CallToolResult) {
	var id identity
	if ti := sdkauth.TokenInfoFromContext(ctx); ti != nil {
		id.userID = ti.UserID
		if e, _ := ti.Extra[auth.ExtraEmail].(string); e != "" {
			id.email = &e
		}
		if t, _ := ti.Extra[auth.ExtraToken].(string); t != "" {
			id.token = t
		}
	} else if s.testUserID != "" {
		id.userID = s.testUserID
	}

	if id.userID == "" {
		return id, jsonErr("Non authentifié.")
	}
	if retry := s.rl.retryAfter(id.userID, time.Now()); retry > 0 {
		return id, jsonErr(fmt.Sprintf("Limite de débit atteinte (%d appels/min) — attends ~%ds avant de réessayer.", s.rlPerMin, retry))
	}
	return id, nil
}

/* ── read tools ────────────────────────────────────────────────────────────── */

func (s *toolServer) whoami(ctx context.Context, _ *mcp.CallToolRequest, _ argsWhoami) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	return jsonOK(outWhoami{UserID: id.userID, Email: id.email}), nil, nil
}

func (s *toolServer) listMatches(ctx context.Context, _ *mcp.CallToolRequest, a argsListMatches) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	ms, err := s.api.Matches(ctx, id.token, comp(a.Competition), a.Status, clampLimit(a.Limit, 200, 200))
	if err != nil {
		return apiErr("list_matches", err), nil, nil
	}
	out := make([]outMatch, 0, len(ms))
	for _, m := range ms {
		out = append(out, projectMatch(m))
	}
	return jsonOK(out), nil, nil
}

func (s *toolServer) getMatch(ctx context.Context, _ *mcp.CallToolRequest, a argsGetMatch) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	m, found, err := s.api.Match(ctx, id.token, a.MatchID)
	if err != nil {
		return apiErr("get_match", err), nil, nil
	}
	if !found {
		return jsonErr("Match introuvable."), nil, nil
	}
	return jsonOK(projectMatchDetail(*m)), nil, nil
}

func (s *toolServer) getMyStanding(ctx context.Context, _ *mcp.CallToolRequest, a argsComp) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	c := comp(a.Competition)
	me, found, err := s.api.Me(ctx, id.token, c)
	if err != nil {
		return apiErr("get_my_standing", err), nil, nil
	}
	if !found {
		return jsonErr("Profil introuvable."), nil, nil
	}
	return jsonOK(projectStanding(*me, c)), nil, nil
}

func (s *toolServer) getMyPredictions(ctx context.Context, _ *mcp.CallToolRequest, a argsPredictions) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	when := a.When
	if when == "" {
		when = "all"
	}
	ps, err := s.api.Predictions(ctx, id.token, comp(a.Competition), when, clampLimit(a.Limit, 100, 200))
	if err != nil {
		return apiErr("get_my_predictions", err), nil, nil
	}
	return jsonOK(projectPredictions(ps)), nil, nil
}

func (s *toolServer) getLeaderboard(ctx context.Context, _ *mcp.CallToolRequest, a argsLeaderboard) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	rows, err := s.api.Leaderboard(ctx, id.token, comp(a.Competition), a.GroupID, clampLimit(a.Limit, 50, 200))
	if err != nil {
		return apiErr("get_leaderboard", err), nil, nil
	}
	return jsonOK(projectLeaderboard(rows)), nil, nil
}

func (s *toolServer) listMyGroups(ctx context.Context, _ *mcp.CallToolRequest, a argsComp) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	gs, err := s.api.Groups(ctx, id.token, comp(a.Competition))
	if err != nil {
		return apiErr("list_my_groups", err), nil, nil
	}
	return jsonOK(projectGroups(gs)), nil, nil
}

func (s *toolServer) getJokerStatus(ctx context.Context, _ *mcp.CallToolRequest, a argsComp) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	js, err := s.api.Jokers(ctx, id.token, comp(a.Competition))
	if err != nil {
		return apiErr("get_joker_status", err), nil, nil
	}
	return jsonOK(projectJokers(js)), nil, nil
}

func (s *toolServer) search(ctx context.Context, _ *mcp.CallToolRequest, a argsSearch) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	res, err := s.api.Search(ctx, id.token, a.Query)
	if err != nil {
		return apiErr("search", err), nil, nil
	}
	return jsonOK(projectSearch(res)), nil, nil
}

/* ── write tool (stub until M4) ────────────────────────────────────────────── */

func (s *toolServer) submitPrediction(ctx context.Context, _ *mcp.CallToolRequest, a argsSubmit) (*mcp.CallToolResult, any, error) {
	id, errRes := s.gate(ctx)
	if errRes != nil {
		return errRes, nil, nil
	}
	if !s.enableWrites {
		return jsonErr("L'écriture de pronos via MCP n'est pas encore disponible sur cette instance (MCP_ENABLE_WRITES=false)."), nil, nil
	}
	// footics-api POST /v1/predictions is the single write point + trust boundary:
	// it re-verifies the token and enforces every rule (score range, 90' lock,
	// joker quota, KO-qualifier normalisation). We forward and surface its verdict.
	in := apiclient.SubmitPredictionInput{
		MatchID: a.MatchID,
		Home:    a.HomeScore,
		Away:    a.AwayScore,
		Joker:   a.Joker,
	}
	if a.WinnerTeamCode != "" {
		w := a.WinnerTeamCode
		in.WinnerTeamCode = &w
	}
	body, status, err := s.api.SubmitPrediction(ctx, id.token, in)
	if err != nil {
		return apiErr("submit_prediction", err), nil, nil
	}
	if status < 200 || status >= 300 {
		// Surface the API's own FR message ({ok:false, error}) verbatim to the model.
		if msg, _ := body["error"].(string); msg != "" {
			return jsonErr(msg), nil, nil
		}
		return jsonErr(fmt.Sprintf("Échec de l'enregistrement du prono (HTTP %d).", status)), nil, nil
	}
	return jsonOK(body), nil, nil
}

/* ── helpers ───────────────────────────────────────────────────────────────── */

// comp defaults an empty competition to "wc" (the schema default already fills
// it; this is belt-and-braces for authless/direct calls).
func comp(c string) string {
	if c == "" {
		return "wc"
	}
	return c
}

// clampLimit mirrors Math.min(Math.max(limit ?? def, 1), max): an absent (0)
// limit uses def, otherwise it is clamped to [1, max].
func clampLimit(v, def, max int) int {
	if v <= 0 {
		v = def
	}
	if v < 1 {
		v = 1
	}
	if v > max {
		v = max
	}
	return v
}

func jsonOK(data any) *mcp.CallToolResult {
	text, err := marshalPretty(data)
	if err != nil {
		return jsonErr("Erreur interne de sérialisation.")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func jsonErr(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: marshalCompact(map[string]string{"error": msg})}},
		IsError: true,
	}
}

// apiErr logs the underlying /v1 failure and returns a generic FR tool error (the
// frozen contract never surfaced API internals; it hit the DB directly).
func apiErr(tool string, err error) *mcp.CallToolResult {
	slog.Error("[mcp] /v1 call failed", "tool", tool, "err", err)
	return jsonErr("Erreur lors de l'appel à l'API Footics. Réessaie plus tard.")
}

// marshalPretty reproduces JSON.stringify(data, null, 2): 2-space indent with
// HTML escaping OFF (JS does not escape < > &).
func marshalPretty(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// marshalCompact reproduces JSON.stringify(data) for the error envelope.
func marshalCompact(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(b.String(), "\n")
}
