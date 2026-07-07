package apiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SubmitPrediction forwards the body + Bearer to footics-api and passes the
// decoded response and status straight back (the API is the trust boundary).
func TestSubmitPredictionForwards(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotCT string
	var gotBody SubmitPredictionInput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth, gotCT = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"prediction":{"home":2,"away":1,"joker":false}}`))
	}))
	defer srv.Close()

	win := "FRA"
	body, status, err := New(srv.URL).SubmitPrediction(context.Background(), "tok-abc", SubmitPredictionInput{
		MatchID: "m1", Home: 2, Away: 1, Joker: false, WinnerTeamCode: &win,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/predictions" {
		t.Fatalf("request = %s %s, want POST /v1/predictions", gotMethod, gotPath)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotBody.MatchID != "m1" || gotBody.Home != 2 || gotBody.Away != 1 || gotBody.WinnerTeamCode == nil || *gotBody.WinnerTeamCode != "FRA" {
		t.Errorf("forwarded body = %+v", gotBody)
	}
	if status != http.StatusOK || body["ok"] != true {
		t.Errorf("status=%d body=%v", status, body)
	}
}

// A non-2xx from the API is passed through verbatim (status + {ok:false,error})
// so the tool layer can surface the API's own FR message.
func TestSubmitPredictionPassesThroughError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error":"Match verrouillé (coup d'envoi passé)."}`))
	}))
	defer srv.Close()

	body, status, err := New(srv.URL).SubmitPrediction(context.Background(), "t", SubmitPredictionInput{MatchID: "m1", Home: 0, Away: 0})
	if err != nil {
		t.Fatalf("transport error should be nil for an HTTP error: %v", err)
	}
	if status != http.StatusConflict {
		t.Errorf("status = %d, want 409", status)
	}
	if body["error"] != "Match verrouillé (coup d'envoi passé)." {
		t.Errorf("error message not passed through: %v", body["error"])
	}
}
