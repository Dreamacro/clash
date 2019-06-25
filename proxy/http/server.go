package http

import (
	"bufio"
	"encoding/base64"
	"github.com/Dreamacro/clash/common/cache"
	"net"
	"net/http"
	"strings"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/component/auth"
	C "github.com/Dreamacro/clash/constant"
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
}

func NewHttpProxy(addr string) (*HttpListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	hl := &HttpListener{l, addr, false}

	go func() {
		log.Infoln("HTTP proxy listening at: %s", addr)
		authCache := cache.New(30 * time.Second)
		for {
			c, err := hl.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}
			go handleConn(c, auth.Authenticator(), authCache)
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

func handleAuth(conn net.Conn) {
	_, err := conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n"))
	if err != nil {
		return
	}
}

func doAuth(loginStr string, auth C.Authenticator, cache *cache.Cache) (ret bool) {
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

func handleConn(conn net.Conn, auth C.Authenticator, cache *cache.Cache) {
	br := bufio.NewReader(conn)
	request, err := http.ReadRequest(br)
	if err != nil || request.URL.Host == "" {
		conn.Close()
		return
	}

	authStrings := strings.Split(request.Header.Get("Proxy-Authorization"), " ")
	if auth.Enabled() && len(authStrings) != 2 {
		handleAuth(conn)
		return
	} else {
		if !doAuth(authStrings[1], auth, cache) {
			conn.Close()
			return
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
