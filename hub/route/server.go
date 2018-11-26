package route

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Dreamacro/clash/log"
	T "github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

var (
	serverSecret = ""
	serverAddr   = ""
)

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func Start(addr string, secret string) {
	if serverAddr != "" {
		return
	}

	serverAddr = addr
	serverSecret = secret

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	})

	r.Use(cors.Handler, authentication)

	r.With(jsonContentType).Get("/traffic", traffic)
	r.With(jsonContentType).Get("/logs", getLogs)
	r.Mount("/configs", configRouter())
	r.Mount("/proxies", proxyRouter())
	r.Mount("/rules", ruleRouter())

	log.Infoln("RESTful API listening at: %s", addr)
	err := http.ListenAndServe(addr, r)
	if err != nil {
		log.Errorln("External controller error: %s", err.Error())
	}
}

func jsonContentType(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func authentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		text := strings.SplitN(header, " ", 2)

		if serverSecret == "" {
			next.ServeHTTP(w, r)
			return
		}

		hasUnvalidHeader := text[0] != "Bearer"
		hasUnvalidSecret := len(text) == 2 && text[1] != serverSecret
		if hasUnvalidHeader || hasUnvalidSecret {
			w.WriteHeader(http.StatusUnauthorized)
			render.Respond(w, r, ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func traffic(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	tick := time.NewTicker(time.Second)
	t := T.Instance().Traffic()
	for range tick.C {
		up, down := t.Now()
		if err := json.NewEncoder(w).Encode(Traffic{
			Up:   up,
			Down: down,
		}); err != nil {
			break
		}
		w.(http.Flusher).Flush()
	}
}

type Log struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

func getLogs(w http.ResponseWriter, r *http.Request) {
	levelText := r.URL.Query().Get("level")
	if levelText == "" {
		levelText = "info"
	}

	level, ok := log.LogLevelMapping[levelText]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		render.Respond(w, r, ErrBadRequest)
		return
	}

	sub := log.Subscribe()
	render.Status(r, http.StatusOK)
	for elm := range sub {
		log := elm.(*log.Event)
		if log.LogLevel < level {
			continue
		}

		if err := json.NewEncoder(w).Encode(Log{
			Type:    log.Type(),
			Payload: log.Payload,
		}); err != nil {
			break
		}
		w.(http.Flusher).Flush()
	}
}
