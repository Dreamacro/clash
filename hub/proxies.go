package hub

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	A "github.com/Dreamacro/clash/adapters/outbound"
	C "github.com/Dreamacro/clash/constant"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func proxyRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getProxies)
	r.With(parseProxyName).Get("/{name}", getProxy)
	r.With(parseProxyName).Get("/{name}/delay", getProxyDelay)
	r.With(parseProxyName).Put("/{name}", updateProxy)
	return r
}

// When name is composed of a partial escape string, Golang does not unescape it
func parseProxyName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if newName, err := url.PathUnescape(name); err == nil {
			name = newName
		}
		ctx := context.WithValue(r.Context(), contextKey("proxy name"), name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type SampleProxy struct {
	Type string `json:"type"`
}

type Selector struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type URLTest struct {
	Type string `json:"type"`
	Now  string `json:"now"`
}

type Fallback struct {
	Type string `json:"type"`
	Now  string `json:"now"`
}

func transformProxy(proxy C.Proxy) interface{} {
	t := proxy.Type()
	switch t {
	case C.Selector:
		selector := proxy.(*A.Selector)
		return Selector{
			Type: t.String(),
			Now:  selector.Now(),
			All:  selector.All(),
		}
	case C.URLTest:
		return URLTest{
			Type: t.String(),
			Now:  proxy.(*A.URLTest).Now(),
		}
	case C.Fallback:
		return Fallback{
			Type: t.String(),
			Now:  proxy.(*A.Fallback).Now(),
		}
	default:
		return SampleProxy{
			Type: proxy.Type().String(),
		}
	}
}

type GetProxiesResponse struct {
	Proxies map[string]interface{} `json:"proxies"`
}

func getProxies(w http.ResponseWriter, r *http.Request) {
	rawProxies := cfg.Proxies()
	proxies := make(map[string]interface{})
	for name, proxy := range rawProxies {
		proxies[name] = transformProxy(proxy)
	}
	render.JSON(w, r, GetProxiesResponse{Proxies: proxies})
}

func getProxy(w http.ResponseWriter, r *http.Request) {
	name := r.Context().Value(contextKey("proxy name")).(string)
	proxies := cfg.Proxies()
	proxy, exist := proxies[name]
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		render.JSON(w, r, Error{
			Error: "Proxy not found",
		})
		return
	}
	render.JSON(w, r, transformProxy(proxy))
}

type UpdateProxyRequest struct {
	Name string `json:"name"`
}

func updateProxy(w http.ResponseWriter, r *http.Request) {
	req := UpdateProxyRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, Error{
			Error: "Format error",
		})
		return
	}

	name := r.Context().Value(contextKey("proxy name")).(string)
	proxies := cfg.Proxies()
	proxy, exist := proxies[name]
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		render.JSON(w, r, Error{
			Error: "Proxy not found",
		})
		return
	}

	selector, ok := proxy.(*A.Selector)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, Error{
			Error: "Proxy can't update",
		})
		return
	}

	if err := selector.Set(req.Name); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, Error{
			Error: fmt.Sprintf("Selector update error: %s", err.Error()),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type GetProxyDelayRequest struct {
	URL     string `json:"url"`
	Timeout int16  `json:"timeout"`
}

type GetProxyDelayResponse struct {
	Delay int16 `json:"delay"`
}

func getProxyDelay(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	url := query.Get("url")
	timeout, err := strconv.ParseInt(query.Get("timeout"), 10, 16)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, Error{
			Error: "Format error",
		})
		return
	}

	name := r.Context().Value(contextKey("proxy name")).(string)
	proxies := cfg.Proxies()
	proxy, exist := proxies[name]
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		render.JSON(w, r, Error{
			Error: "Proxy not found",
		})
		return
	}

	sigCh := make(chan int16)
	go func() {
		t, err := A.DelayTest(proxy, url)
		if err != nil {
			sigCh <- 0
		}
		sigCh <- t
	}()

	select {
	case <-time.After(time.Millisecond * time.Duration(timeout)):
		w.WriteHeader(http.StatusRequestTimeout)
		render.JSON(w, r, Error{
			Error: "Proxy delay test timeout",
		})
	case t := <-sigCh:
		if t == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			render.JSON(w, r, Error{
				Error: "An error occurred in the delay test",
			})
		} else {
			render.JSON(w, r, GetProxyDelayResponse{
				Delay: t,
			})
		}
	}
}
