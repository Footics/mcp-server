package tools

// shapes.go — the FROZEN MCP output shapes and the projections from the /v1
// view-model into them. These structs (field names, order, and null semantics)
// are the public contract that existing MCP connectors depend on; they mirror
// lib/tools.ts + lib/queries.ts of the TS server exactly.
//
// Null semantics: a "T | null" frozen field is a pointer WITHOUT omitempty, so a
// nil marshals to `null` (present key), matching JSON.stringify of the TS
// view-model. Slices are pre-allocated non-nil so an empty result marshals to
// `[]`, never `null`.

import (
	"time"

	"github.com/footics/mcp-server/internal/apiclient"
)

/* ── whoami ────────────────────────────────────────────────────────────────── */

type outWhoami struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
}

/* ── match (list_matches / get_match) ──────────────────────────────────────── */

type outTeam struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type outScore struct {
	Home int `json:"home"`
	Away int `json:"away"`
}

type outLive struct {
	Period  string `json:"period"`
	Minute  *int   `json:"minute"`
	Running bool   `json:"running"`
}

type outMyPred struct {
	Home   int  `json:"home"`
	Away   int  `json:"away"`
	Joker  bool `json:"joker"`
	Points *int `json:"points"`
}

type outMatch struct {
	ID           string     `json:"id"`
	Competition  string     `json:"competition"`
	Phase        string     `json:"phase"`
	Group        *string    `json:"group"`
	Status       string     `json:"status"`
	KickoffAt    string     `json:"kickoffAt"`
	Venue        *string    `json:"venue"`
	Home         outTeam    `json:"home"`
	Away         outTeam    `json:"away"`
	Score        *outScore  `json:"score"`
	Live         *outLive   `json:"live"`
	MyPrediction *outMyPred `json:"myPrediction"`
}

type outEvent struct {
	Type       string  `json:"type"`
	Period     string  `json:"period"`
	Minute     int     `json:"minute"`
	MinutePlus *int    `json:"minutePlus"`
	TeamCode   *string `json:"teamCode"`
	Player     *string `json:"player"`
	Detail     *string `json:"detail"`
}

// outMatchDetail is outMatch + events (events appended last, mirroring the TS
// spread `{...base, events}`).
type outMatchDetail struct {
	outMatch
	Events []outEvent `json:"events"`
}

func projectMatch(m apiclient.Match) outMatch {
	// score: present only for live|finished. TS reads the persisted home/away
	// score column: Score for finished, LiveScore for live.
	var score *outScore
	switch m.Status {
	case "finished":
		score = fromScore(m.Score)
	case "live":
		score = fromScore(m.LiveScore)
	}

	var live *outLive
	if m.Status == "live" && m.Live != nil {
		live = &outLive{Period: m.Live.Period, Minute: m.Live.DisplayMinute, Running: m.Live.Running}
	}

	var pred *outMyPred
	if m.Mine != nil {
		joker := false
		if m.JokerApplied != nil {
			joker = *m.JokerApplied
		}
		pred = &outMyPred{Home: m.Mine.H, Away: m.Mine.A, Joker: joker, Points: m.Points}
	}

	return outMatch{
		ID:           m.ID,
		Competition:  m.Competition.Kind,
		Phase:        m.Phase,
		Group:        m.Group,
		Status:       m.Status,
		KickoffAt:    toISO(m.KickoffAt),
		Venue:        nilIfEmpty(m.Venue),
		Home:         outTeam{Code: m.Home.Code, Name: m.Home.Name},
		Away:         outTeam{Code: m.Away.Code, Name: m.Away.Name},
		Score:        score,
		Live:         live,
		MyPrediction: pred,
	}
}

func projectMatchDetail(d apiclient.MatchDetail) outMatchDetail {
	base := projectMatch(d.Match)
	events := make([]outEvent, 0, len(d.Events)) // [] not null
	if base.Status != "scheduled" {              // TS: [] before a match starts
		for _, e := range d.Events {
			events = append(events, outEvent{
				Type: e.Type, Period: e.Period, Minute: e.Minute,
				MinutePlus: e.MinutePlus, TeamCode: e.TeamCode, Player: e.Player, Detail: e.Detail,
			})
		}
	}
	return outMatchDetail{outMatch: base, Events: events}
}

/* ── get_my_standing ───────────────────────────────────────────────────────── */

type outStanding struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	Competition string `json:"competition"`
	Points      int    `json:"points"`
	Exact       int    `json:"exact"`
	Rank        int    `json:"rank"`
	RankOf      int    `json:"rankOf"`
}

func projectStanding(me apiclient.Me, comp string) outStanding {
	return outStanding{
		UserID: me.ID, Username: me.Username, Competition: comp,
		Points: me.Points, Exact: me.Exact, Rank: me.Rank, RankOf: me.RankOf,
	}
}

/* ── get_my_predictions ────────────────────────────────────────────────────── */

type outPredScore struct {
	Home  int  `json:"home"`
	Away  int  `json:"away"`
	Joker bool `json:"joker"`
}

type outPrediction struct {
	MatchID    string       `json:"matchId"`
	Fixture    string       `json:"fixture"`
	KickoffAt  string       `json:"kickoffAt"`
	Status     string       `json:"status"`
	Prediction outPredScore `json:"prediction"`
	Result     *outScore    `json:"result"`
	Points     *int         `json:"points"`
}

func projectPredictions(ps []apiclient.Prediction) []outPrediction {
	out := make([]outPrediction, 0, len(ps))
	for _, p := range ps {
		var result *outScore
		if p.Result != nil {
			result = &outScore{Home: p.Result.Home, Away: p.Result.Away}
		}
		out = append(out, outPrediction{
			MatchID:    p.MatchID,
			Fixture:    p.Fixture,
			KickoffAt:  toISO(p.KickoffAt),
			Status:     p.Status,
			Prediction: outPredScore{Home: p.Prediction.Home, Away: p.Prediction.Away, Joker: p.Prediction.Joker},
			Result:     result,
			Points:     p.Points,
		})
	}
	return out
}

/* ── get_leaderboard ───────────────────────────────────────────────────────── */

type outLeaderRow struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Points   int    `json:"points"`
	Exact    int    `json:"exact"`
	Me       bool   `json:"me"`
}

func projectLeaderboard(rows []apiclient.LeaderRow) []outLeaderRow {
	out := make([]outLeaderRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, outLeaderRow{Rank: r.Rank, Username: r.Username, Points: r.Points, Exact: r.Exact, Me: r.Me})
	}
	return out
}

/* ── list_my_groups ────────────────────────────────────────────────────────── */

type outGroup struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Members  int    `json:"members"`
	MyRank   int    `json:"myRank"`
	MyPoints int    `json:"myPoints"`
	Owner    bool   `json:"owner"`
}

func projectGroups(gs []apiclient.Group) []outGroup {
	out := make([]outGroup, 0, len(gs))
	for _, g := range gs {
		out = append(out, outGroup{
			ID: g.ID, Name: g.Name, Code: g.Code, Members: g.Members,
			MyRank: g.MyRank, MyPoints: g.MyPoints, Owner: g.Owner != nil,
		})
	}
	return out
}

/* ── get_joker_status ──────────────────────────────────────────────────────── */

type outJoker struct {
	Bucket    string `json:"bucket"`
	Label     string `json:"label"`
	Used      int    `json:"used"`
	Quota     int    `json:"quota"`
	Remaining int    `json:"remaining"`
}

func projectJokers(js []apiclient.Joker) []outJoker {
	out := make([]outJoker, 0, len(js))
	for _, j := range js {
		out = append(out, outJoker{Bucket: j.Bucket, Label: j.Label, Used: j.Used, Quota: j.Quota, Remaining: j.Remaining})
	}
	return out
}

/* ── search ────────────────────────────────────────────────────────────────── */

type outSearchMatch struct {
	ID        string `json:"id"`
	Fixture   string `json:"fixture"`
	KickoffAt string `json:"kickoffAt"`
}

type outSearchGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type outSearchUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Me       bool   `json:"me"`
}

type outSearch struct {
	Matches []outSearchMatch `json:"matches"`
	Groups  []outSearchGroup `json:"groups"`
	Users   []outSearchUser  `json:"users"`
}

func projectSearch(res apiclient.SearchResults) outSearch {
	out := outSearch{
		Matches: make([]outSearchMatch, 0, len(res.Matches)),
		Groups:  make([]outSearchGroup, 0, len(res.Groups)),
		Users:   make([]outSearchUser, 0, len(res.Users)),
	}
	for _, m := range res.Matches {
		out.Matches = append(out.Matches, outSearchMatch{ID: m.ID, Fixture: m.Fixture, KickoffAt: toISO(m.KickoffAt)})
	}
	for _, g := range res.Groups {
		out.Groups = append(out.Groups, outSearchGroup{ID: g.ID, Name: g.Name})
	}
	for _, u := range res.Users {
		out.Users = append(out.Users, outSearchUser{ID: u.ID, Username: u.Username, Me: u.Me})
	}
	return out
}

/* ── helpers ───────────────────────────────────────────────────────────────── */

func fromScore(s *apiclient.Score) *outScore {
	if s == nil {
		return nil
	}
	return &outScore{Home: s.H, Away: s.A}
}

// nilIfEmpty maps the API's empty-string venue back to null, reproducing the TS
// `venue ?? null` for an absent venue (a real venue is always a non-empty name).
// ponytail: this collapses a genuinely-empty venue to null too — impossible in
// the data, and the frozen contract's type is `string | null`.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// toISO normalises a /v1 timestamp to the exact JS `Date.toISOString()` form
// (UTC, always 3 fractional digits, `Z`). /v1/matches already emits millis, but
// /v1/predictions and /v1/search use RFC3339 without them — the frozen contract
// (built from toISOString) always has `.000`. Unparseable input is passed
// through untouched rather than dropped.
func toISO(s string) string {
	t, err := time.Parse(time.RFC3339, s) // lenient: also accepts a fractional part
	if err != nil {
		return s
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
