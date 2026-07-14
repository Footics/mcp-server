package tools

// schemas.go — the FROZEN input schemas, fixed EXPLICITLY (not inferred from Go
// structs) so tools/list advertises the same params, enums, bounds, defaults and
// FR descriptions as the zod-derived TS schemas. The SDK applies schema defaults
// (competition→"wc", when→"all", joker→false) before validating and unmarshaling,
// so absent optional args resolve exactly as they did in the TS server.

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

// typed argument structs (only used to unmarshal the defaults-applied arguments;
// the wire schema is the explicit one set on each Tool).
type (
	argsWhoami      struct{}
	argsListMatches struct {
		Competition string `json:"competition"`
		Status      string `json:"status"`
		Limit       int    `json:"limit"`
	}
	argsGetMatch struct {
		MatchID string `json:"matchId"`
	}
	argsComp struct {
		Competition string `json:"competition"`
	}
	argsPredictions struct {
		Competition string `json:"competition"`
		When        string `json:"when"`
		Limit       int    `json:"limit"`
	}
	argsLeaderboard struct {
		Competition string `json:"competition"`
		GroupID     string `json:"groupId"`
		Limit       int    `json:"limit"`
	}
	argsSearch struct {
		Query string `json:"query"`
	}
	argsSubmit struct {
		MatchID        string `json:"matchId"`
		HomeScore      int    `json:"homeScore"`
		AwayScore      int    `json:"awayScore"`
		Joker          bool   `json:"joker"`
		WinnerTeamCode string `json:"winnerTeamCode"`
	}
)

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

func rawDefault(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func object(props map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Properties: props, Required: required}
}

// competitionSchema is the shared `competition` param (enum + default wc).
func competitionSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Enum:        []any{"wc", "friendlies"},
		Default:     rawDefault("wc"),
		Description: `Compétition : "wc" (Coupe du monde) ou "friendlies" (amicaux). Défaut: wc.`,
	}
}

func schemaWhoami() *jsonschema.Schema { return object(map[string]*jsonschema.Schema{}) }

func schemaListMatches() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"competition": competitionSchema(),
		"status": {
			Type:        "string",
			Enum:        []any{"scheduled", "live", "finished", "postponed", "cancelled"},
			Description: "Filtre de statut optionnel.",
		},
		"limit": {Type: "integer", Minimum: f64(1), Maximum: f64(200), Description: "Nombre max de matchs (défaut 200)."},
	})
}

func schemaGetMatch() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"matchId": {Type: "string", MinLength: iptr(1), Description: "L'id (uuid) du match."},
	}, "matchId")
}

func schemaComp() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{"competition": competitionSchema()})
}

func schemaPredictions() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"competition": competitionSchema(),
		"when": {
			Type:    "string",
			Enum:    []any{"upcoming", "past", "all"},
			Default: rawDefault("all"),
		},
		"limit": {Type: "integer", Minimum: f64(1), Maximum: f64(200)},
	})
}

func schemaLeaderboard() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"competition": competitionSchema(),
		"groupId":     {Type: "string", Description: "Id d'un groupe dont je suis membre (sinon classement général)."},
		"limit":       {Type: "integer", Minimum: f64(1), Maximum: f64(200)},
	})
}

func schemaSearch() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"query": {Type: "string", MinLength: iptr(1), Description: "Texte recherché."},
	}, "query")
}

func schemaSubmit() *jsonschema.Schema {
	return object(map[string]*jsonschema.Schema{
		"matchId":   {Type: "string", MinLength: iptr(1), Description: "L'id (uuid) du match — voir list_matches/search."},
		"homeScore": {Type: "integer", Minimum: f64(0), Maximum: f64(20), Description: "Score prédit de l'équipe à domicile (0-20)."},
		"awayScore": {Type: "integer", Minimum: f64(0), Maximum: f64(20), Description: "Score prédit de l'équipe à l'extérieur (0-20)."},
		"joker":     {Type: "boolean", Default: rawDefault(false), Description: "Appliquer un joker (double les points). Défaut: false."},
		"winnerTeamCode": {
			Type:        "string",
			MinLength:   iptr(2),
			MaxLength:   iptr(3),
			Description: "Match à élimination directe + nul prédit UNIQUEMENT : code FIFA-3 de l'équipe qui se qualifie (pour le +1). Doit être l'une des 2 équipes. Ignoré sur un score décisif ou en phase de poules.",
		},
	}, "matchId", "homeScore", "awayScore")
}
