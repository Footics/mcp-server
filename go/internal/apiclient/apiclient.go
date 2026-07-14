// Package apiclient is a thin HTTP client of footics-api /v1. It relays the
// caller's Bearer verbatim (footics-api re-verifies as the trust boundary and
// scopes the data) and adds ?competition= and the per-tool query params. It has
// NO database access; footics-api owns all reads.
//
// The response structs decode only the fields the MCP tools project into their
// frozen output shapes; extra /v1 view-model fields are ignored.
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// requestTimeout bounds each /v1 call. The API is one internal hop in prod.
const requestTimeout = 10 * time.Second

// Client calls footics-api /v1.
type Client struct {
	base string
	hc   *http.Client
}

// New builds a client for the given base URL (e.g. http://api:8080).
func New(baseURL string) *Client {
	return &Client{base: baseURL, hc: &http.Client{Timeout: requestTimeout}}
}

/* ── /v1 response shapes (only the projected fields) ───────────────────────── */

type Score struct {
	H int `json:"h"`
	A int `json:"a"`
}

type Team struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type LiveClock struct {
	Period        string `json:"period"`
	DisplayMinute *int   `json:"displayMinute"`
	Running       bool   `json:"running"`
}

// Match mirrors the /v1 matchJSON fields the MCP needs.
type Match struct {
	ID          string  `json:"id"`
	Phase       string  `json:"phase"`
	Group       *string `json:"group"`
	Status      string  `json:"status"`
	KickoffAt   string  `json:"kickoffAt"`
	Venue       string  `json:"venue"`
	Competition struct {
		Kind string `json:"kind"`
	} `json:"competition"`
	Home         Team       `json:"home"`
	Away         Team       `json:"away"`
	Score        *Score     `json:"score"`
	LiveScore    *Score     `json:"liveScore"`
	Live         *LiveClock `json:"live"`
	Mine         *Score     `json:"mine"`
	JokerApplied *bool      `json:"jokerApplied"`
	Points       *int       `json:"points"`
}

type Event struct {
	Type       string  `json:"type"`
	Period     string  `json:"period"`
	Minute     int     `json:"minute"`
	MinutePlus *int    `json:"minutePlus"`
	TeamCode   *string `json:"teamCode"`
	Player     *string `json:"player"`
	Detail     *string `json:"detail"`
}

// MatchDetail is a Match plus its event timeline.
type MatchDetail struct {
	Match
	Events []Event `json:"events"`
}

// Me mirrors the /v1/me MeProfile subset the MCP projects.
type Me struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Points   int    `json:"points"`
	Exact    int    `json:"exact"`
	Rank     int    `json:"rank"`
	RankOf   int    `json:"rankOf"`
}

type Prediction struct {
	MatchID    string `json:"matchId"`
	Fixture    string `json:"fixture"`
	KickoffAt  string `json:"kickoffAt"`
	Status     string `json:"status"`
	Prediction struct {
		Home  int  `json:"home"`
		Away  int  `json:"away"`
		Joker bool `json:"joker"`
	} `json:"prediction"`
	Result *struct {
		Home int `json:"home"`
		Away int `json:"away"`
	} `json:"result"`
	Points *int `json:"points"`
}

type LeaderRow struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Points   int    `json:"points"`
	Exact    int    `json:"exact"`
	Me       bool   `json:"me"`
}

type Group struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Members  int     `json:"members"`
	MyRank   int     `json:"myRank"`
	MyPoints int     `json:"myPoints"`
	Owner    *string `json:"owner"`
}

type Joker struct {
	Bucket    string `json:"bucket"`
	Label     string `json:"label"`
	Used      int    `json:"used"`
	Quota     int    `json:"quota"`
	Remaining int    `json:"remaining"`
}

type SearchResults struct {
	Matches []struct {
		ID        string `json:"id"`
		Fixture   string `json:"fixture"`
		KickoffAt string `json:"kickoffAt"`
	} `json:"matches"`
	Groups []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"groups"`
	Users []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Me       bool   `json:"me"`
	} `json:"users"`
}

/* ── typed calls ───────────────────────────────────────────────────────────── */

// Matches → GET /v1/matches?competition=&status=&limit=.
func (c *Client) Matches(ctx context.Context, token, comp, status string, limit int) ([]Match, error) {
	q := url.Values{"competition": {comp}, "limit": {strconv.Itoa(limit)}}
	if status != "" {
		q.Set("status", status)
	}
	var out []Match
	_, err := c.getJSON(ctx, token, "/v1/matches", q, &out)
	return out, err
}

// Match → GET /v1/matches/{id}. found=false on 404.
func (c *Client) Match(ctx context.Context, token, id string) (*MatchDetail, bool, error) {
	var out MatchDetail
	code, err := c.getJSON(ctx, token, "/v1/matches/"+url.PathEscape(id), nil, &out)
	if code == http.StatusNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &out, true, nil
}

// Me → GET /v1/me?competition=. found=false on 404.
func (c *Client) Me(ctx context.Context, token, comp string) (*Me, bool, error) {
	var out struct {
		Me *Me `json:"me"`
	}
	code, err := c.getJSON(ctx, token, "/v1/me", url.Values{"competition": {comp}}, &out)
	if code == http.StatusNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if out.Me == nil {
		return nil, false, nil
	}
	return out.Me, true, nil
}

// Predictions → GET /v1/predictions?competition=&when=&limit=.
func (c *Client) Predictions(ctx context.Context, token, comp, when string, limit int) ([]Prediction, error) {
	q := url.Values{"competition": {comp}, "when": {when}, "limit": {strconv.Itoa(limit)}}
	var out []Prediction
	_, err := c.getJSON(ctx, token, "/v1/predictions", q, &out)
	return out, err
}

// Leaderboard → GET /v1/leaderboard?competition=&group=&limit=; returns rows.
func (c *Client) Leaderboard(ctx context.Context, token, comp, groupID string, limit int) ([]LeaderRow, error) {
	q := url.Values{"competition": {comp}, "limit": {strconv.Itoa(limit)}}
	if groupID != "" {
		q.Set("group", groupID)
	}
	var out struct {
		Rows []LeaderRow `json:"rows"`
	}
	_, err := c.getJSON(ctx, token, "/v1/leaderboard", q, &out)
	return out.Rows, err
}

// Groups → GET /v1/groups?competition=; returns the caller's groups.
func (c *Client) Groups(ctx context.Context, token, comp string) ([]Group, error) {
	var out struct {
		Groups []Group `json:"groups"`
	}
	_, err := c.getJSON(ctx, token, "/v1/groups", url.Values{"competition": {comp}}, &out)
	return out.Groups, err
}

// Jokers → GET /v1/jokers?competition=.
func (c *Client) Jokers(ctx context.Context, token, comp string) ([]Joker, error) {
	var out []Joker
	_, err := c.getJSON(ctx, token, "/v1/jokers", url.Values{"competition": {comp}}, &out)
	return out, err
}

// Search → GET /v1/search?q=.
func (c *Client) Search(ctx context.Context, token, query string) (SearchResults, error) {
	var out SearchResults
	_, err := c.getJSON(ctx, token, "/v1/search", url.Values{"q": {query}}, &out)
	return out, err
}

// getJSON does an authenticated GET and decodes a 2xx body into dst. It returns
// the HTTP status (so callers can special-case 404) and a non-nil error on any
// non-2xx status or transport/decode failure.
func (c *Client) getJSON(ctx context.Context, token, path string, q url.Values, dst any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return 0, err
	}
	if q != nil {
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("footics-api GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return resp.StatusCode, fmt.Errorf("footics-api GET %s: decode: %w", path, err)
	}
	return resp.StatusCode, nil
}
