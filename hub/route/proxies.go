package route

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	A "github.com/Dreamacro/clash/adapters/outbound"
	C "github.com/Dreamacro/clash/constant"
	T "github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func proxyRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getProxies)

	r.Route("/{name}", func(r chi.Router) {
		r.Use(parseProxyName, findProxyByName)
		r.Get("/", getProxy)
		r.Get("/delay", getProxyDelay)
		r.Get("/healthcheck", checkProxyHealth)
		r.Put("/", updateProxy)
	})
	return r
}

// When name is composed of a partial escape string, Golang does not unescape it
func parseProxyName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if newName, err := url.PathUnescape(name); err == nil {
			name = newName
		}
		ctx := context.WithValue(r.Context(), CtxKeyProxyName, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func findProxyByName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.Context().Value(CtxKeyProxyName).(string)
		proxies := T.Instance().Proxies()
		proxy, exist := proxies[name]
		if !exist {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, ErrNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), CtxKeyProxy, proxy)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getProxies(w http.ResponseWriter, r *http.Request) {
	proxies := T.Instance().Proxies()
	render.JSON(w, r, render.M{
		"proxies": proxies,
	})
}

func getProxy(w http.ResponseWriter, r *http.Request) {
	proxy := r.Context().Value(CtxKeyProxy).(C.Proxy)
	render.JSON(w, r, proxy)
}

type UpdateProxyRequest struct {
	Name string `json:"name"`
}

func updateProxy(w http.ResponseWriter, r *http.Request) {
	req := UpdateProxyRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	proxy := r.Context().Value(CtxKeyProxy).(*A.Proxy)
	selector, ok := proxy.ProxyAdapter.(*A.Selector)
	if !ok {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, newError("Must be a Selector"))
		return
	}

	if err := selector.Set(req.Name); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, newError(fmt.Sprintf("Selector update error: %s", err.Error())))
		return
	}

	render.NoContent(w, r)
}

func getProxyDelay(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	url := query.Get("url")
	timeout, err := strconv.ParseInt(query.Get("timeout"), 10, 16)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	proxy := r.Context().Value(CtxKeyProxy).(C.Proxy)

	back := A.WithGroupKey(context.Background(), A.MakeGroupKey(proxy.Name(), url, timeout*time.Millisecond.Nanoseconds()))
	ctx, cancel := context.WithTimeout(back, time.Millisecond*time.Duration(timeout))
	defer cancel()

	delay, err := proxy.HealthCheck(ctx, url)
	if ctx.Err() != nil {
		render.Status(r, http.StatusGatewayTimeout)
		render.JSON(w, r, ErrRequestTimeout)
		return
	}

	if err != nil || delay == 0 {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, newError("An error occurred in the delay test"))
		return
	}

	render.JSON(w, r, render.M{
		"delay": delay,
	})
}

func checkProxyHealth(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	timeout, err := strconv.ParseInt(query.Get("timeout"), 10, 16)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	url := query.Get("url")

	proxy := r.Context().Value(CtxKeyProxy).(*A.Proxy)

	back := A.WithGroupKey(context.Background(), A.MakeGroupKey(proxy.Name(), url, timeout*time.Millisecond.Nanoseconds()))
	ctx, cancel := context.WithTimeout(back, time.Millisecond*time.Duration(timeout))
	defer cancel()

	t, err := proxy.HealthCheck(ctx, url)
	if err != nil {
		render.JSON(w, r, render.M{
			"health": false,
			"err":    err.Error(),
		})
		return
	}

	resp := render.M{
		"health": true,
		"delay":  t,
	}

	// additional info.
	switch p := proxy.ProxyAdapter.(type) {
	case *A.URLTest:
		resp["now"] = p.Now()
	}
	render.JSON(w, r, resp)
}
