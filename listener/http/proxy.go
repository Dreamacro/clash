package http

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter/inbound"
	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/auth"
	C "github.com/Dreamacro/clash/constant"
	authStore "github.com/Dreamacro/clash/listener/auth"
	"github.com/Dreamacro/clash/log"
)

type Proxy struct {
	client       *http.Client
	cache        *cache.Cache
	in           chan<- C.ConnContext
	authenticate bool
}

func (p *Proxy) ServeConn(conn net.Conn) {
	reader := bufio.NewReader(conn)

	keepAlive := true
	activated := !p.authenticate

	for keepAlive {
		request, err := ReadRequest(reader, false)
		if err != nil {
			break
		}

		keepAlive = strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive"

		var resp *http.Response

		if !activated {
			authenticator := authStore.Authenticator()
			if authenticator != nil {
				if authStrings := strings.Split(request.Header.Get("Proxy-Authorization"), " "); len(authStrings) != 2 {
					resp = &http.Response{
						StatusCode: http.StatusProxyAuthRequired,
						Status:     http.StatusText(http.StatusProxyAuthRequired),
						Proto:      "HTTP/1.1",
						ProtoMajor: 1,
						ProtoMinor: 1,
						Request:    request,
						Header: http.Header{
							"Proxy-Authenticate": []string{"Basic"},
						},
					}
				} else if !p.canActivate(authStrings[1], authenticator) {
					log.Infoln("Auth failed from %s", conn.RemoteAddr().String())

					resp = &http.Response{
						StatusCode: http.StatusForbidden,
						Status:     http.StatusText(http.StatusForbidden),
						Proto:      "HTTP/1.1",
						ProtoMajor: 1,
						ProtoMinor: 1,
						Request:    request,
						Header:     http.Header{},
					}
				} else {
					activated = true
				}
			} else {
				activated = true
			}
		}

		if activated {
			if request.Method == http.MethodConnect {
				_, err = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
				if err != nil {
					conn.Close()
					return
				}

				p.in <- inbound.NewHTTPS(request, &httpsConn{conn, reader})

				return
			}

			host := request.Header.Get("Host")
			if host != "" {
				request.Host = host
			}

			RemoveHopByHopHeaders(request.Header)
			RemoveExtraHTTPHostPort(request)
			request.RequestURI = ""

			if request.URL.Scheme == "" || request.URL.Host == "" {
				resp = &http.Response{
					StatusCode: http.StatusBadRequest,
					Status:     http.StatusText(http.StatusBadRequest),
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Request:    request,
					Header:     http.Header{},
				}
			} else {
				resp, err = p.client.Do(request.WithContext(context.WithValue(request.Context(), remoteAddrKey, conn.RemoteAddr())))
				if err != nil {
					resp = &http.Response{
						StatusCode: http.StatusBadGateway,
						Status:     http.StatusText(http.StatusBadGateway),
						Proto:      "HTTP/1.1",
						ProtoMajor: 1,
						ProtoMinor: 1,
						Request:    request,
						Body:       io.NopCloser(strings.NewReader(err.Error())),
						Header: http.Header{
							"Content-Type": []string{"text/plain"},
						},
					}
				}
			}
		}

		if resp == nil {
			break
		}

		// close conn when header `Connection` is `close`
		if strings.ToLower(resp.Header.Get("Connection")) == "close" {
			keepAlive = false
		}

		if keepAlive {
			resp.Header.Set("Proxy-Connection", "keep-alive")
			resp.Header.Set("Connection", "keep-alive")
			resp.Header.Set("Keep-Alive", "timeout=4")
			resp.Close = false
		} else {
			resp.Close = true
		}
		err = resp.Write(conn)
		if err != nil || resp.Close {
			break
		}
	}

	conn.Close()
}

func (p *Proxy) canActivate(loginStr string, authenticator auth.Authenticator) (ret bool) {
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
	return NewProxyWithAuthenticate(in, cache, true)
}

func NewProxyWithAuthenticate(in chan<- C.ConnContext, cache *cache.Cache, authenticate bool) *Proxy {
	return &Proxy{
		client:       newClient(in),
		cache:        cache,
		in:           in,
		authenticate: authenticate,
	}
}
