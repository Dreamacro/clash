package gun

import (
	"context"
	"errors"
	"net"

	"github.com/Qv2ray/gun-lite/pkg/realgun"
)

type Config struct {
	ServiceName    string
	SkipCertVerify bool
	Tls            bool
	ServerName     string
	Adder          string
}

func StreamGunConn(cfg *Config) (net.Conn, error) {
	client := realgun.NewGunClientWithContext(context.TODO(), &realgun.Config{
		RemoteAddr:  cfg.Adder,
		ServerName:  cfg.ServerName,
		ServiceName: cfg.ServiceName,
		Cleartext:   !cfg.Tls,
	})
	gConn, err := client.DialConn()
	if err != nil {
		return nil, errors.New("failed to dial remote: " + err.Error())
	}
	return gConn, nil
}
