package hub

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func ruleRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getRules)
	r.Put("/", updateRules)
	return r
}

type Rule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

type GetRulesResponse struct {
	Rules []Rule `json:"rules"`
}

func getRules(w http.ResponseWriter, r *http.Request) {
	rawRules := cfg.Rules()

	var rules []Rule
	for _, rule := range rawRules {
		rules = append(rules, Rule{
			Type:    rule.RuleType().String(),
			Payload: rule.Payload(),
			Proxy:   rule.Adapter(),
		})
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, GetRulesResponse{
		Rules: rules,
	})
}

func updateRules(w http.ResponseWriter, r *http.Request) {
	err := cfg.UpdateRules()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, Error{
			Error: err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
