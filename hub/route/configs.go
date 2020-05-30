package route

import (
	"net/http"
	"path/filepath"

	"github.com/Dreamacro/clash/config"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/log"
	P "github.com/Dreamacro/clash/proxy"
	"github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func configRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConfigs)
	r.Put("/", reloadConfigs)
	r.Patch("/", patchConfigs)
	return r
}

type configSchema struct {
	Port        *int               `json:"port"`
	SocksPort   *int               `json:"socks-port"`
	RedirPort   *int               `json:"redir-port"`
	AllowLan    *bool              `json:"allow-lan"`
	BindAddress *string            `json:"bind-address"`
	Mode        *tunnel.TunnelMode `json:"mode"`
	LogLevel    *log.LogLevel      `json:"log-level"`
}

func getConfigs(w http.ResponseWriter, r *http.Request) {
	general := executor.GetGeneral()
	render.JSON(w, r, general)
}

func pointerOrDefault(p *int, def int) int {
	if p != nil {
		return *p
	}

	return def
}

func patchConfigs(w http.ResponseWriter, r *http.Request) {
	general := &configSchema{}
	if err := render.DecodeJSON(r.Body, general); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	if general.AllowLan != nil {
		P.SetAllowLan(*general.AllowLan)
	}

	if general.BindAddress != nil {
		P.SetBindAddress(*general.BindAddress)
	}

	ports := P.GetPorts()
	P.ReCreateHTTP(pointerOrDefault(general.Port, ports.Port))
	P.ReCreateSocks(pointerOrDefault(general.SocksPort, ports.SocksPort))
	P.ReCreateRedir(pointerOrDefault(general.RedirPort, ports.RedirPort))

	if general.Mode != nil {
		tunnel.SetMode(*general.Mode)
	}

	if general.LogLevel != nil {
		log.SetLevel(*general.LogLevel)
	}

	render.NoContent(w, r)
}

type updateConfigRequest struct {
	Path    string `json:"path"`
	Payload string `json:"payload"`
}
type reloadConfigRequest struct {
	Force bool   `json:"force"`
	Path  string `json:"path"`
}

func reloadConfigs(w http.ResponseWriter, r *http.Request) {
	req := reloadConfigRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
	}
	var cfg *config.Config
	var err error
	if req.Path != "" {
		if !filepath.IsAbs(req.Path) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("path is not a absoluted path"))
			return
		}
		cfg, err = executor.ParseWithPath(req.Path)

	} else {
		cfg, err = executor.Parse()
	}
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, newError(err.Error()))
		return
	}
	executor.ApplyConfig(cfg, req.Force)
	render.NoContent(w, r)
}
func updateConfigs(w http.ResponseWriter, r *http.Request) {
	req := updateConfigRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	var cfg *config.Config
	var err error

	if req.Payload != "" {
		cfg, err = executor.ParseWithBytes([]byte(req.Payload))
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	} else {
		if !filepath.IsAbs(req.Path) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("path is not a absoluted path"))
			return
		}

		cfg, err = executor.ParseWithPath(req.Path)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	}

	executor.ApplyConfig(cfg, force)
	render.NoContent(w, r)
}
