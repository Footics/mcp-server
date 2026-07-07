package tools

// shapes_test.go — asserts each projection produces the EXACT frozen JSON shape
// (field names, order, null semantics) the TS server emitted. Compact JSON is
// compared byte-for-byte; struct field order is the declaration order.

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/footics/mcp-server/internal/apiclient"
)

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }
func bp(b bool) *bool     { return &b }

func eq(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s:\n got: %s\nwant: %s", name, got, want)
	}
}

func TestProjectMatchFinished(t *testing.T) {
	m := apiclient.Match{
		ID: "m1", Phase: "group", Status: "finished", KickoffAt: "2026-07-04T15:00:00.000Z", Venue: "",
		Home: apiclient.Team{Code: "FRA", Name: "France"}, Away: apiclient.Team{Code: "MAR", Name: "Maroc"},
		Score: &apiclient.Score{H: 2, A: 1}, Mine: &apiclient.Score{H: 2, A: 1}, JokerApplied: bp(false), Points: ip(3),
	}
	m.Competition.Kind = "wc"
	eq(t, "finished", marshalCompact(projectMatch(m)),
		`{"id":"m1","competition":"wc","phase":"group","group":null,"status":"finished","kickoffAt":"2026-07-04T15:00:00.000Z","venue":null,"home":{"code":"FRA","name":"France"},"away":{"code":"MAR","name":"Maroc"},"score":{"home":2,"away":1},"live":null,"myPrediction":{"home":2,"away":1,"joker":false,"points":3}}`)
}

func TestProjectMatchLive(t *testing.T) {
	m := apiclient.Match{
		ID: "m2", Phase: "group", Group: sp("A"), Status: "live", KickoffAt: "2026-07-04T18:00:00Z", Venue: "MetLife Stadium",
		Home: apiclient.Team{Code: "BRA", Name: "Brésil"}, Away: apiclient.Team{Code: "ARG", Name: "Argentine"},
		LiveScore: &apiclient.Score{H: 1, A: 0},
		Live:      &apiclient.LiveClock{Period: "second_half", DisplayMinute: ip(52), Running: true},
		Mine:      &apiclient.Score{H: 1, A: 1}, JokerApplied: bp(true), Points: nil,
	}
	m.Competition.Kind = "wc"
	// kickoffAt normalised to millis; score from liveScore; myPrediction.points null.
	eq(t, "live", marshalCompact(projectMatch(m)),
		`{"id":"m2","competition":"wc","phase":"group","group":"A","status":"live","kickoffAt":"2026-07-04T18:00:00.000Z","venue":"MetLife Stadium","home":{"code":"BRA","name":"Brésil"},"away":{"code":"ARG","name":"Argentine"},"score":{"home":1,"away":0},"live":{"period":"second_half","minute":52,"running":true},"myPrediction":{"home":1,"away":1,"joker":true,"points":null}}`)
}

func TestProjectMatchScheduledNoPred(t *testing.T) {
	m := apiclient.Match{ID: "m3", Phase: "group", Status: "scheduled", KickoffAt: "2026-07-04T20:00:00.000Z",
		Home: apiclient.Team{Code: "ESP", Name: "Espagne"}, Away: apiclient.Team{Code: "GER", Name: "Allemagne"}}
	m.Competition.Kind = "wc"
	eq(t, "scheduled", marshalCompact(projectMatch(m)),
		`{"id":"m3","competition":"wc","phase":"group","group":null,"status":"scheduled","kickoffAt":"2026-07-04T20:00:00.000Z","venue":null,"home":{"code":"ESP","name":"Espagne"},"away":{"code":"GER","name":"Allemagne"},"score":null,"live":null,"myPrediction":null}`)
}

func TestProjectMatchDetailScheduledForcesEmptyEvents(t *testing.T) {
	d := apiclient.MatchDetail{
		Match:  apiclient.Match{ID: "m3", Phase: "group", Status: "scheduled", KickoffAt: "2026-07-04T20:00:00.000Z", Home: apiclient.Team{Code: "ESP", Name: "Espagne"}, Away: apiclient.Team{Code: "GER", Name: "Allemagne"}},
		Events: []apiclient.Event{{Type: "goal", Period: "first_half", Minute: 5}}, // must be dropped: scheduled ⇒ []
	}
	d.Competition.Kind = "wc"
	got := marshalCompact(projectMatchDetail(d))
	want := `{"id":"m3","competition":"wc","phase":"group","group":null,"status":"scheduled","kickoffAt":"2026-07-04T20:00:00.000Z","venue":null,"home":{"code":"ESP","name":"Espagne"},"away":{"code":"GER","name":"Allemagne"},"score":null,"live":null,"myPrediction":null,"events":[]}`
	eq(t, "detail-scheduled", got, want)
}

func TestProjectMatchDetailFinishedDropsAssist(t *testing.T) {
	d := apiclient.MatchDetail{
		Match:  apiclient.Match{ID: "m1", Phase: "group", Status: "finished", KickoffAt: "2026-07-04T15:00:00.000Z", Home: apiclient.Team{Code: "FRA", Name: "France"}, Away: apiclient.Team{Code: "MAR", Name: "Maroc"}, Score: &apiclient.Score{H: 2, A: 1}},
		Events: []apiclient.Event{{Type: "goal", Period: "first_half", Minute: 23, TeamCode: sp("FRA"), Player: sp("Mbappé")}},
	}
	d.Competition.Kind = "wc"
	got := marshalCompact(projectMatchDetail(d))
	// EventJson has no `assist`; minutePlus/detail present as null.
	want := `{"id":"m1","competition":"wc","phase":"group","group":null,"status":"finished","kickoffAt":"2026-07-04T15:00:00.000Z","venue":null,"home":{"code":"FRA","name":"France"},"away":{"code":"MAR","name":"Maroc"},"score":{"home":2,"away":1},"live":null,"myPrediction":null,"events":[{"type":"goal","period":"first_half","minute":23,"minutePlus":null,"teamCode":"FRA","player":"Mbappé","detail":null}]}`
	eq(t, "detail-finished", got, want)
}

func TestProjectPredictions(t *testing.T) {
	ps := []apiclient.Prediction{
		{MatchID: "m1", Fixture: "FRA-MAR", KickoffAt: "2026-07-04T15:00:00Z", Status: "finished",
			Prediction: struct {
				Home  int  `json:"home"`
				Away  int  `json:"away"`
				Joker bool `json:"joker"`
			}{2, 1, false},
			Result: &struct {
				Home int `json:"home"`
				Away int `json:"away"`
			}{2, 1}, Points: ip(3)},
		{MatchID: "m2", Fixture: "BRA-ARG", KickoffAt: "2026-07-04T18:00:00Z", Status: "scheduled",
			Prediction: struct {
				Home  int  `json:"home"`
				Away  int  `json:"away"`
				Joker bool `json:"joker"`
			}{3, 0, false}},
	}
	// prediction has NO winnerTeamCode; result/points present-null when absent; kickoffAt normalised.
	want := `[{"matchId":"m1","fixture":"FRA-MAR","kickoffAt":"2026-07-04T15:00:00.000Z","status":"finished","prediction":{"home":2,"away":1,"joker":false},"result":{"home":2,"away":1},"points":3},{"matchId":"m2","fixture":"BRA-ARG","kickoffAt":"2026-07-04T18:00:00.000Z","status":"scheduled","prediction":{"home":3,"away":0,"joker":false},"result":null,"points":null}]`
	eq(t, "predictions", marshalCompact(projectPredictions(ps)), want)
}

func TestProjectGroupsOwnerFlag(t *testing.T) {
	gs := []apiclient.Group{
		{ID: "g1", Name: "Les Potes", Code: "ABC123", Members: 5, MyRank: 2, MyPoints: 12, Owner: sp("me")},
		{ID: "g2", Name: "Boulot", Code: "XYZ", Members: 3, MyRank: 3, MyPoints: 0, Owner: nil},
	}
	want := `[{"id":"g1","name":"Les Potes","code":"ABC123","members":5,"myRank":2,"myPoints":12,"owner":true},{"id":"g2","name":"Boulot","code":"XYZ","members":3,"myRank":3,"myPoints":0,"owner":false}]`
	eq(t, "groups", marshalCompact(projectGroups(gs)), want)
}

func TestProjectSearchEmptyIsArrays(t *testing.T) {
	// empty categories marshal to [], not null.
	eq(t, "search-empty", marshalCompact(projectSearch(apiclient.SearchResults{})), `{"matches":[],"groups":[],"users":[]}`)
}

func TestToISO(t *testing.T) {
	cases := map[string]string{
		"2026-07-04T18:00:00Z":     "2026-07-04T18:00:00.000Z",
		"2026-07-04T15:00:00.000Z": "2026-07-04T15:00:00.000Z",
		"2026-07-04T15:00:00.42Z":  "2026-07-04T15:00:00.420Z",
		"not-a-date":               "not-a-date", // unparseable passes through
	}
	for in, want := range cases {
		if got := toISO(in); got != want {
			t.Errorf("toISO(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONEnvelopes(t *testing.T) {
	if txt := text(t, jsonErr("Match introuvable.")); txt != `{"error":"Match introuvable."}` {
		t.Errorf("jsonErr text = %q", txt)
	}
	if !jsonErr("x").IsError {
		t.Error("jsonErr must set IsError")
	}
	// jsonOK is pretty-printed (2-space indent) and does not HTML-escape.
	if txt := text(t, jsonOK(map[string]string{"a": "<b>&"})); txt != "{\n  \"a\": \"<b>&\"\n}" {
		t.Errorf("jsonOK text = %q", txt)
	}
	if jsonOK(map[string]string{}).IsError {
		t.Error("jsonOK must not set IsError")
	}
}

func text(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) != 1 {
		t.Fatalf("want 1 content block, got %d", len(r.Content))
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content is not TextContent: %T", r.Content[0])
	}
	return tc.Text
}
