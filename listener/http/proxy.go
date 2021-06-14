package http

import (
    "encoding/base64"
    "io"
    "net"
    "net/http"
    "strings"
    "time"

    "github.com/Dreamacro/clash/adapter/inbound"
    "github.com/Dreamacro/clash/common/cache"
    N "github.com/Dreamacro/clash/common/net"
    "github.com/Dreamacro/clash/component/auth"
    C "github.com/Dreamacro/clash/constant"
    authStore "github.com/Dreamacro/clash/listener/auth"
    "github.com/Dreamacro/clash/log"
)

func HandleConn(c net.Conn, in chan<- C.ConnContext, cache *cache.Cache) {
    client := newClient(c.RemoteAddr(), in)
    conn := N.NewBufferedConn(c)

    keepAlive := true
    activated := cache == nil // disable authenticate if cache is nil

    for keepAlive {
        request, err := ReadRequest(conn.Reader(), false)
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
                } else if !canActivate(authStrings[1], authenticator, cache) {
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

                in <- inbound.NewHTTPS(request, conn)

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
                resp, err = client.Do(request)
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

func canActivate(loginStr string, authenticator auth.Authenticator, cache *cache.Cache) (ret bool) {
    if result := cache.Get(loginStr); result != nil {
        ret = result.(bool)
        return
    }
    loginData, err := base64.StdEncoding.DecodeString(loginStr)
    login := strings.Split(string(loginData), ":")
    ret = err == nil && len(login) == 2 && authenticator.Verify(login[0], login[1])

    cache.Put(loginStr, ret, time.Minute)
    return
}
