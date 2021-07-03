package obfs

import (
	"errors"
	"fmt"
	"net"
)

var (
	errTLS12TicketAuthIncorrectMagicNumber = errors.New("tls1.2_ticket_auth incorrect magic number")
	errTLS12TicketAuthTooShortData         = errors.New("tls1.2_ticket_auth too short data")
	errTLS12TicketAuthHMACError            = errors.New("tls1.2_ticket_auth hmac verifying failed")
)

type authData struct {
	clientID [32]byte
}

type Obfs interface {
	StreamConn(net.Conn) net.Conn
}

type obfsCreator func(b *Base) Obfs

var obfsList = make(map[string]struct {
	overhead int
	new      obfsCreator
})

func register(name string, c obfsCreator, o int) {
	obfsList[name] = struct {
		overhead int
		new      obfsCreator
	}{overhead: o, new: c}
}

func PickObfs(name string, b *Base) (Obfs, int, error) {
	if choice, ok := obfsList[name]; ok {
		return choice.new(b), choice.overhead, nil
	}
	return nil, 0, fmt.Errorf("Obfs %s not supported", name)
}
