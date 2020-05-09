package mixed

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/cache"
	"github.com/Dreamacro/clash/component/auth"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	authStore "github.com/Dreamacro/clash/proxy/auth"
	"github.com/Dreamacro/clash/tunnel"
)

type MixedListener struct {
	net.Listener
	address string
	closed  bool
	cache   *cache.Cache
}

const socks5_Version = 5

func NewMixedProxy(addr string) (*MixedListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	ml := &MixedListener{l, addr, false, cache.New(30 * time.Second)}
	go func() {
		log.Infoln("Mixed(http+socks5) proxy listening at: %s", addr)

		for {
			c, err := ml.Accept()
			if err != nil {
				if ml.closed {
					break
				}
				continue
			}
			go handleConn(c, ml.cache)
		}
	}()

	return ml, nil
}

func (l *MixedListener) Close() {
	l.closed = true
	l.Listener.Close()
}

func (l *MixedListener) Address() string {
	return l.address
}

func canActivate(loginStr string, authenticator auth.Authenticator, cache *cache.Cache) (ret bool) {
	if result := cache.Get(loginStr); result != nil {
		ret = result.(bool)
	}
	loginData, err := base64.StdEncoding.DecodeString(loginStr)
	login := strings.Split(string(loginData), ":")
	ret = err == nil && len(login) == 2 && authenticator.Verify(login[0], login[1])

	cache.Put(loginStr, ret, time.Minute)
	return
}

func handleSocks(conn net.Conn) {
	target, command, err := socks5.ServerHandshake(conn, authStore.Authenticator())
	if err != nil {
		conn.Close()
		return
	}

	// conn.(*net.TCPConn).SetKeepAlive(true)
	if command == socks5.CmdUDPAssociate {
		defer conn.Close()
		io.Copy(ioutil.Discard, conn)
		return
	}
	tunnel.Add(adapters.NewSocket(target, conn, C.SOCKS))
}

func handleConn(conn net.Conn, cache *cache.Cache) {
	if c, ok := conn.(*net.TCPConn); ok {
		c.SetKeepAlive(true)
	}
	cex := NewConnEx(conn)
	head, err := cex.Peek(1)
	if err != nil {
		return
	}
	if head[0] == socks5_Version {
		handleSocks(cex)
		return
	}

	request, err := http.ReadRequest(cex.Reader())
	if err != nil || request.URL.Host == "" {
		conn.Close()
		return
	}
	authenticator := authStore.Authenticator()
	if authenticator != nil {
		if authStrings := strings.Split(request.Header.Get("Proxy-Authorization"), " "); len(authStrings) != 2 {
			_, err = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n"))
			conn.Close()
			return
		} else if !canActivate(authStrings[1], authenticator, cache) {
			conn.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
			log.Infoln("Auth failed from %s", conn.RemoteAddr().String())
			conn.Close()
			return
		}
	}

	if request.Method == http.MethodConnect {
		_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			return
		}
		tunnel.Add(adapters.NewHTTPS(request, conn))
		return
	}

	tunnel.Add(adapters.NewHTTP(request, conn))
}
