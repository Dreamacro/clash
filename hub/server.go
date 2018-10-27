package hub

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	T "github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

var secret = ""

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func newHub(signal chan struct{}) {
	var addr string
	ch := config.Instance().Subscribe()
	signal <- struct{}{}
	count := 0
	for {
		elm := <-ch
		event := elm.(*config.Event)
		switch event.Type {
		case "external-controller":
			addr = event.Payload.(string)
			count++
		case "secret":
			secret = event.Payload.(string)
			count++
		}
		if count == 2 {
			break
		}
	}

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

	log.Infof("RESTful API listening at: %s", addr)
	err := http.ListenAndServe(addr, r)
	if err != nil {
		log.Errorf("External controller error: %s", err.Error())
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

		if secret == "" {
			next.ServeHTTP(w, r)
			return
		}

		hasUnvalidHeader := text[0] != "Bearer"
		hasUnvalidSecret := len(text) == 2 && text[1] != secret
		if hasUnvalidHeader || hasUnvalidSecret {
			w.WriteHeader(http.StatusUnauthorized)
			render.JSON(w, r, Error{
				Error: "Authentication failed",
			})
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

type contextKey string

func (c contextKey) String() string {
	return "clash context key " + string(c)
}

func traffic(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	tick := time.NewTicker(time.Second)
	t := tunnel.Traffic()
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

	level, ok := C.LogLevelMapping[levelText]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, Error{
			Error: "Level error",
		})
		return
	}

	src := tunnel.Log()
	sub, err := src.Subscribe()
	defer src.UnSubscribe(sub)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, Error{
			Error: err.Error(),
		})
		return
	}
	render.Status(r, http.StatusOK)
	for elm := range sub {
		log := elm.(T.Log)
		if log.LogLevel > level {
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

// Run initial hub
func Run() {
	signal := make(chan struct{})
	go newHub(signal)
	<-signal
}
