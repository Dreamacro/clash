package http

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter/inbound"
	"github.com/Dreamacro/clash/common/cache"
	N "github.com/Dreamacro/clash/common/net"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/auth"
	C "github.com/Dreamacro/clash/constant"
	authStore "github.com/Dreamacro/clash/listener/auth"
	"github.com/Dreamacro/clash/log"
)

type proxy struct {
	listener *listener
	client   *http.Client
	cache    *cache.Cache
	in       chan<- C.ConnContext
}

type Proxy struct {
	*proxy
}

func (p *proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	authenticator := authStore.Authenticator()
	if authenticator != nil {
		if authStrings := strings.Split(request.Header.Get("Proxy-Authorization"), " "); len(authStrings) != 2 {
			writer.Header().Set("Proxy-Authenticate", "Basic")
			writer.WriteHeader(http.StatusProxyAuthRequired)
			return
		} else if !p.canActivate(authStrings[1], authenticator) {
			writer.WriteHeader(http.StatusForbidden)
			log.Infoln("Auth failed from %s", request.RemoteAddr)
			return
		}
	}

	if request.Method == http.MethodConnect {
		dropBody(request)

		conn, buf, err := writer.(http.Hijacker).Hijack()
		if err != nil {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}

		conn.SetDeadline(time.Time{})

		err = buf.Flush()
		if err != nil {
			conn.Close()
			return
		}

		_, err = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			conn.Close()
			return
		}

		p.in <- inbound.NewHTTPS(request, &hijackedConn{conn, buf})

		return
	}

	inbound.RemoveHopByHopHeaders(request.Header)
	inbound.RemoveExtraHTTPHostPort(request)
	request.RequestURI = ""

	resp, err := p.client.Do(request.WithContext(context.WithValue(request.Context(), remoteAddrKey, request.RemoteAddr)))
	if err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	inbound.RemoveHopByHopHeaders(resp.Header)
	copyHeader(writer.Header(), resp.Header)
	writer.WriteHeader(resp.StatusCode)

	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)

	io.CopyBuffer(N.WriteOnlyWriter{Writer: writer}, N.ReadOnlyReader{Reader: resp.Body}, buf)
}

func (p *proxy) ServeConn(conn net.Conn) {
	p.listener.Inject(conn)
}

func (p *proxy) canActivate(loginStr string, authenticator auth.Authenticator) (ret bool) {
	if result := p.cache.Get(loginStr); result != nil {
		ret = result.(bool)
		return
	}
	loginData, err := base64.StdEncoding.DecodeString(loginStr)
	login := strings.Split(string(loginData), ":")
	ret = err == nil && len(login) == 2 && authenticator.Verify(login[0], login[1])

	p.cache.Put(loginStr, ret, time.Minute)
	return
}

func NewProxy(in chan<- C.ConnContext, cache *cache.Cache) *Proxy {
	p := &proxy{
		listener: newListener(),
		client:   newClient(in),
		cache:    cache,
		in:       in,
	}

	r := &Proxy{p}

	go http.Serve(p.listener, p)

	runtime.SetFinalizer(r, destroyProxy)

	return r
}

func dropBody(req *http.Request) {
	defer req.Body.Close()

	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)

	for {
		_, err := io.ReadFull(req.Body, buf)
		if err != nil {
			return
		}
	}
}

func copyHeader(dst http.Header, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}

func destroyProxy(p *Proxy) {
	p.listener.Close()
}
