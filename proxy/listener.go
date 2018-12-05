package proxy

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/proxy/http"
	"github.com/Dreamacro/clash/proxy/redir"
	"github.com/Dreamacro/clash/proxy/socks"
)

var (
	allowLan = false

	socksListener *socks.SockListener
	httpListener  *http.HttpListener
	redirListener *redir.RedirListener
)

type listener interface {
	Close()
	Address() string
}

type Ports struct {
	Port      int `json:"port"`
	SocksPort int `json:"socks-port"`
	RedirPort int `json:"redir-port"`
}

func AllowLan() bool {
	return allowLan
}

func SetAllowLan(al bool) {
	allowLan = al
}

func ReCreateHTTP(port int) error {
	addr := genAddr(port, allowLan)

	if httpListener != nil {
		if httpListener.Address() == addr {
			return nil
		}
		httpListener.Close()
		httpListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	httpListener, err = http.NewHttpProxy(addr)
	if err != nil {
		return err
	}

	return nil
}

func ReCreateSocks(port int) error {
	addr := genAddr(port, allowLan)

	if socksListener != nil {
		if socksListener.Address() == addr {
			return nil
		}
		socksListener.Close()
		socksListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	socksListener, err = socks.NewSocksProxy(addr)
	if err != nil {
		return err
	}

	return nil
}

func ReCreateRedir(port int) error {
	addr := genAddr(port, allowLan)

	if redirListener != nil {
		if redirListener.Address() == addr {
			return nil
		}
		redirListener.Close()
		redirListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	redirListener, err = redir.NewRedirProxy(addr)
	if err != nil {
		return err
	}

	return nil
}

// GetPorts return the ports of proxy servers
func GetPorts() *Ports {
	ports := &Ports{}

	if httpListener != nil {
		_, portStr, _ := net.SplitHostPort(httpListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.Port = port
	}

	if socksListener != nil {
		_, portStr, _ := net.SplitHostPort(socksListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.SocksPort = port
	}

	if redirListener != nil {
		_, portStr, _ := net.SplitHostPort(redirListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.RedirPort = port
	}

	return ports
}

func portIsZero(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return true
	}
	return false
}

func genAddr(port int, allowLan bool) string {
	if allowLan {
		return fmt.Sprintf(":%d", port)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}
