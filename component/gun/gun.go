package gun

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/Dreamacro/clash/component/gun/proto"
	C "github.com/Dreamacro/clash/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

var (
	globalDialerMap    map[string]*grpc.ClientConn
	globalDialerAccess sync.Mutex
)

type Config struct {
	Host           string
	ServiceName    string
	SkipCertVerify bool
	Tls            bool
	Port           string
	ServerName     string
}

func getGunClient(cfg *Config, metadata *C.Metadata, dialOption grpc.DialOption) (*grpc.ClientConn, error) {
	globalDialerAccess.Lock()
	defer globalDialerAccess.Unlock()

	if globalDialerMap == nil {
		globalDialerMap = make(map[string]*grpc.ClientConn)
	}
	falg := metadata.RemoteAddress()
	if client, found := globalDialerMap[falg]; found && client.GetState() != connectivity.Shutdown {
		return client, nil
	}

	conn, err := grpc.Dial(
		net.JoinHostPort(cfg.Host, cfg.Port),
		dialOption,
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  500 * time.Millisecond,
				Multiplier: 1.5,
				Jitter:     0.2,
				MaxDelay:   19 * time.Millisecond,
			},
			MinConnectTimeout: 5 * time.Second,
		}),
	)
	globalDialerMap[falg] = conn
	return conn, err
}

func StreamGunConn(metadata *C.Metadata, cfg *Config, ctx context.Context) (net.Conn, error) {
	dialOption := grpc.WithInsecure()
	if cfg.Tls {
		tlsConfig := &tls.Config{ServerName: cfg.ServerName, InsecureSkipVerify: cfg.SkipCertVerify}
		if cfg.ServerName != "" {
			tlsConfig.ServerName = cfg.ServerName
		} else {
			tlsConfig.ServerName = cfg.Host
		}
		dialOption = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	}
	gConn, err := getGunClient(cfg, metadata, dialOption)
	if err != nil {
		return nil, err
	}
	client := proto.NewGunServiceClient(gConn)
	grpcservice, err := client.(proto.GunServiceClientX).TunCustomName(ctx, cfg.ServiceName)
	if err != nil {
		return nil, err
	}
	clientConn := proto.NewClientConn(grpcservice)
	return clientConn, nil
}
