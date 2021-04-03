package route

import (
	"net/http"

	"github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func ruleRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getRules)
	return r
}

type Rule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

func getRules(w http.ResponseWriter, r *http.Request) {
	rawRules := tunnel.Rules()

	rules := []Rule{}
	for _, rule := range rawRules {
		rules = append(rules, Rule{
			Type:    rule.RuleType().String(),
			Payload: rule.Payload(),
			Proxy:   rule.Adapter(),
		})
	}

	render.JSON(w, r, render.M{
		"rules": rules,
	})
}
