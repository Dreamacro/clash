package http

import (
	"bufio"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/auth"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"
)

var (
	tun = tunnel.Instance()
)

type HttpListener struct {
	net.Listener
	address string
	closed  bool
	cache   *cache.Cache
}

func NewHttpProxy(addr string, authenticator auth.Authenticator) (*HttpListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	hl := &HttpListener{l, addr, false, cache.New(30 * time.Second)}

	go func() {
		log.Infoln("HTTP proxy listening at: %s", addr)

		for {
			c, err := hl.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}
			go handleConn(c, authenticator, hl.cache)
		}
	}()

	return hl, nil
}

func (l *HttpListener) Close() {
	l.closed = true
	l.Listener.Close()
}

func (l *HttpListener) Address() string {
	return l.address
}

func doAuth(loginStr string, auth auth.Authenticator, cache *cache.Cache) (ret bool) {
	if result := cache.Get(loginStr); result == nil {
		loginData, err := base64.StdEncoding.DecodeString(loginStr)
		login := strings.Split(string(loginData), ":")
		if err != nil || len(login) != 2 || !auth.Verify(login[0], login[1]) {
			ret = false
		}
		ret = true
	} else {
		ret = result.(bool)
	}

	cache.Put(loginStr, ret, time.Minute)
	return
}

func handleConn(conn net.Conn, auth auth.Authenticator, cache *cache.Cache) {
	br := bufio.NewReader(conn)
	request, err := http.ReadRequest(br)
	if err != nil || request.URL.Host == "" {
		conn.Close()
		return
	}

	if auth != nil {
		if authStrings := strings.Split(request.Header.Get("Proxy-Authorization"), " "); len(authStrings) != 2 {
			_, err = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n"))
			return
		} else {
			if !doAuth(authStrings[1], auth, cache) {
				log.Infoln("Auth failed from %s", conn.RemoteAddr().String())
				conn.Close()
				return
			}
		}
	}

	if request.Method == http.MethodConnect {
		_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			return
		}
		tun.Add(adapters.NewHTTPS(request, conn))
		return
	}

	tun.Add(adapters.NewHTTP(request, conn))
}
