package route

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	T "github.com/Dreamacro/clash/tunnel"
	"github.com/gorilla/websocket"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func connectionRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConnections)
	r.Delete("/", closeAllConnections)
	r.Delete("/{id}", closeConnection)
	return r
}

func getConnections(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		snapshot := T.DefaultManager.Snapshot()
		render.JSON(w, r, snapshot)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	intervalStr := r.URL.Query().Get("interval")
	interval := 1000
	if intervalStr != "" {
		t, err := strconv.Atoi(intervalStr)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, ErrBadRequest)
			return
		}

		interval = t
	}

	buf := &bytes.Buffer{}
	sendSnapshot := func() error {
		buf.Reset()
		snapshot := T.DefaultManager.Snapshot()
		if err := json.NewEncoder(buf).Encode(snapshot); err != nil {
			return err
		}

		return conn.WriteMessage(websocket.TextMessage, buf.Bytes())
	}

	if err := sendSnapshot(); err != nil {
		return
	}

	tick := time.NewTicker(time.Millisecond * time.Duration(interval))
	for range tick.C {
		if err := sendSnapshot(); err != nil {
			break
		}
	}
}

func closeConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snapshot := T.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		if id == c.ID() {
			c.Close()
			break
		}
	}
	render.NoContent(w, r)
}

func closeAllConnections(w http.ResponseWriter, r *http.Request) {
	snapshot := T.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		c.Close()
	}
	render.NoContent(w, r)
}
