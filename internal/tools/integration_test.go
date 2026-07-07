package tools

// integration_test.go — end-to-end against the running staging footics-api
// (http://127.0.0.1:8099). It stands up the REAL stack (RequireBearerToken →
// Streamable HTTP → tools → /v1 relay), connects a go-sdk MCP client carrying a
// minted alice Bearer, invokes every read tool, and asserts the frozen shape on
// live data. Skips cleanly when the staging API is unreachable.
//
// Run: go test ./internal/tools/ -run TestIntegration -v

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/footics/mcp-server/internal/apiclient"
	"github.com/footics/mcp-server/internal/auth"
	"github.com/footics/mcp-server/internal/config"
)

const (
	stagingURL    = "http://127.0.0.1:8099"
	stagingSecret = "dev-staging-hs256-secret-000000"
	aliceSub      = "3da4cf94-51f9-416f-9e0e-532ca33fa2a5"
	aliceEmail    = "alice@staging.test"
)

// bearerRT injects the caller's Bearer on every client request (the client
// transport has no header hook otherwise).
type bearerRT struct {
	token string
	base  http.RoundTripper
}

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r2)
}

func mintAlice(t *testing.T) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": aliceSub, "aud": "authenticated", "email": aliceEmail,
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(stagingSecret))
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return s
}

func stagingReachable() bool {
	c := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get(stagingURL + "/healthz")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// session stands up the real MCP stack in front of the staging API and returns a
// connected client session.
func session(t *testing.T) (*mcp.ClientSession, context.Context) {
	t.Helper()
	cfg := config.Config{
		JWTSecret: stagingSecret, JWTAud: "authenticated",
		APIBaseURL: stagingURL, PublicURL: "http://mcp.test", Resource: "http://mcp.test/mcp",
		RequireAuth: true, Enabled: true, RateLimitPerMin: 0,
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "footics-mcp-test", Version: "test"}, nil)
	Register(server, Deps{API: apiclient.New(cfg.APIBaseURL), RateLimitPerMin: 0})

	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
	authed := sdkauth.RequireBearerToken(auth.NewVerifier(cfg).Verify,
		&sdkauth.RequireBearerTokenOptions{ResourceMetadataURL: cfg.Resource})(streamable)

	ts := httptest.NewServer(authed)
	t.Cleanup(ts.Close)

	hc := &http.Client{Transport: bearerRT{token: mintAlice(t), base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{Endpoint: ts.URL, HTTPClient: hc, DisableStandaloneSSE: true}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess, ctx
}

// call invokes a tool and returns its decoded (non-error) JSON payload.
func call(t *testing.T, sess *mcp.ClientSession, ctx context.Context, name string, args map[string]any) any {
	t.Helper()
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: transport error: %v", name, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("%s: want 1 content block, got %d", name, len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s: content not text: %T", name, res.Content[0])
	}
	if res.IsError {
		t.Fatalf("%s: tool error: %s", name, tc.Text)
	}
	var v any
	if err := json.Unmarshal([]byte(tc.Text), &v); err != nil {
		t.Fatalf("%s: bad JSON: %v\n%s", name, err, tc.Text)
	}
	return v
}

func TestIntegrationReadTools(t *testing.T) {
	if !stagingReachable() {
		t.Skipf("staging API not reachable at %s — skipping", stagingURL)
	}
	sess, ctx := session(t)

	// tools/list — all 10 tools present.
	lt, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range lt.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"whoami", "list_matches", "get_match", "get_my_standing", "get_my_predictions", "get_leaderboard", "list_my_groups", "get_joker_status", "search", "submit_prediction"} {
		if !names[want] {
			t.Errorf("tools/list missing %q", want)
		}
	}
	if len(lt.Tools) != 10 {
		t.Errorf("tools/list count = %d, want 10", len(lt.Tools))
	}

	// whoami — identity straight from the JWT.
	who := call(t, sess, ctx, "whoami", nil).(map[string]any)
	if who["userId"] != aliceSub {
		t.Errorf("whoami userId = %v", who["userId"])
	}
	if who["email"] != aliceEmail {
		t.Errorf("whoami email = %v", who["email"])
	}

	// list_matches — flat array; FRA-MAR present with alice's prediction.
	matches := call(t, sess, ctx, "list_matches", map[string]any{"competition": "wc"}).([]any)
	if len(matches) == 0 {
		t.Fatal("list_matches empty")
	}
	fraMar := findMatch(matches, "FRA", "MAR")
	if fraMar == nil {
		t.Fatal("list_matches: FRA-MAR not found")
	}
	assertMatchShape(t, fraMar)
	if fraMar["myPrediction"] == nil {
		t.Error("FRA-MAR: alice's myPrediction should be present")
	}
	matchID, _ := fraMar["id"].(string)

	// get_match — detail carries an events array.
	detail := call(t, sess, ctx, "get_match", map[string]any{"matchId": matchID}).(map[string]any)
	assertMatchShape(t, detail)
	if _, ok := detail["events"].([]any); !ok {
		t.Errorf("get_match: events not an array: %T", detail["events"])
	}

	// get_my_standing — alice's standing for wc.
	st := call(t, sess, ctx, "get_my_standing", map[string]any{"competition": "wc"}).(map[string]any)
	for _, k := range []string{"userId", "username", "competition", "points", "exact", "rank", "rankOf"} {
		if _, ok := st[k]; !ok {
			t.Errorf("get_my_standing missing %q", k)
		}
	}
	if st["userId"] != aliceSub || st["username"] != "alice" {
		t.Errorf("get_my_standing identity = %v/%v", st["userId"], st["username"])
	}

	// get_my_predictions — prediction has {home,away,joker} and NO winnerTeamCode.
	preds := call(t, sess, ctx, "get_my_predictions", map[string]any{"competition": "wc", "when": "all"}).([]any)
	if len(preds) == 0 {
		t.Fatal("get_my_predictions empty (alice has predictions in the seed)")
	}
	p0 := preds[0].(map[string]any)
	for _, k := range []string{"matchId", "fixture", "kickoffAt", "status", "prediction", "result", "points"} {
		if _, ok := p0[k]; !ok {
			t.Errorf("prediction missing %q", k)
		}
	}
	pred := p0["prediction"].(map[string]any)
	if _, leaked := pred["winnerTeamCode"]; leaked {
		t.Error("prediction leaked winnerTeamCode (must be dropped in the frozen shape)")
	}
	for _, k := range []string{"home", "away", "joker"} {
		if _, ok := pred[k]; !ok {
			t.Errorf("prediction.%s missing", k)
		}
	}

	// get_leaderboard — flat rows, alice flagged me:true.
	board := call(t, sess, ctx, "get_leaderboard", map[string]any{"competition": "wc"}).([]any)
	if len(board) == 0 {
		t.Fatal("get_leaderboard empty")
	}
	var sawMe bool
	for _, r := range board {
		row := r.(map[string]any)
		for _, k := range []string{"rank", "username", "points", "exact", "me"} {
			if _, ok := row[k]; !ok {
				t.Errorf("leaderboard row missing %q", k)
			}
		}
		if row["me"] == true {
			sawMe = true
		}
	}
	if !sawMe {
		t.Error("get_leaderboard: no row flagged me:true for alice")
	}

	// list_my_groups — array (alice may have none → []).
	if _, ok := call(t, sess, ctx, "list_my_groups", map[string]any{"competition": "wc"}).([]any); !ok {
		t.Error("list_my_groups did not return an array")
	}

	// get_joker_status — buckets; wc has a "group" bucket, quota 12.
	jokers := call(t, sess, ctx, "get_joker_status", map[string]any{"competition": "wc"}).([]any)
	if len(jokers) == 0 {
		t.Fatal("get_joker_status empty")
	}
	var groupBucket map[string]any
	for _, j := range jokers {
		b := j.(map[string]any)
		for _, k := range []string{"bucket", "label", "used", "quota", "remaining"} {
			if _, ok := b[k]; !ok {
				t.Errorf("joker bucket missing %q", k)
			}
		}
		if b["bucket"] == "group" {
			groupBucket = b
		}
	}
	if groupBucket == nil || groupBucket["quota"].(float64) != 12 {
		t.Errorf("get_joker_status: group bucket wrong: %v", groupBucket)
	}

	// search — {matches,groups,users}; FRA-MAR surfaced by "fra".
	sr := call(t, sess, ctx, "search", map[string]any{"query": "fra"}).(map[string]any)
	for _, k := range []string{"matches", "groups", "users"} {
		if _, ok := sr[k].([]any); !ok {
			t.Errorf("search.%s not an array", k)
		}
	}
	if len(sr["matches"].([]any)) == 0 {
		t.Error("search 'fra' found no matches (expected FRA-MAR)")
	}

	// submit_prediction — writes OFF here ⇒ tool error with the FR message.
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "submit_prediction", Arguments: map[string]any{"matchId": matchID, "homeScore": 1, "awayScore": 0}})
	if err != nil {
		t.Fatalf("submit_prediction transport error: %v", err)
	}
	if !res.IsError {
		t.Error("submit_prediction should be a tool error while writes are OFF")
	}
}

func findMatch(matches []any, home, away string) map[string]any {
	for _, m := range matches {
		mm := m.(map[string]any)
		h, _ := mm["home"].(map[string]any)
		a, _ := mm["away"].(map[string]any)
		if h != nil && a != nil && h["code"] == home && a["code"] == away {
			return mm
		}
	}
	return nil
}

func assertMatchShape(t *testing.T, m map[string]any) {
	t.Helper()
	for _, k := range []string{"id", "competition", "phase", "group", "status", "kickoffAt", "venue", "home", "away", "score", "live", "myPrediction"} {
		if _, ok := m[k]; !ok {
			t.Errorf("match missing key %q", k)
		}
	}
	if m["competition"] != "wc" && m["competition"] != "friendlies" {
		t.Errorf("match.competition = %v, want wc|friendlies", m["competition"])
	}
}
