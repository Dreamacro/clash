package gun

import (
	"context"
	"crypto/tls"
	"errors"
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
	ServiceName    string
	SkipCertVerify bool
	Tls            bool
	ServerName     string
	Adder          string
}

func getGunClient(cfg *Config, metadata *C.Metadata, dialOption grpc.DialOption) (*grpc.ClientConn, error) {
	globalDialerAccess.Lock()
	defer globalDialerAccess.Unlock()

	if globalDialerMap == nil {
		globalDialerMap = make(map[string]*grpc.ClientConn)
	}
	falg := metadata.Host + ":" + metadata.RemoteAddress()
	if client, found := globalDialerMap[falg]; found && client.GetState() != connectivity.Shutdown {
		return client, nil
	}

	conn, err := grpc.Dial(
		cfg.Adder,
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
		dialOption = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	}
	gConn, err := getGunClient(cfg, metadata, dialOption)
	if err != nil {
		return nil, errors.New("failed to dial remote: " + err.Error())
	}
	client := proto.NewGunServiceClient(gConn)
	grpcservice, err := client.(proto.GunServiceClientX).TunCustomName(ctx, cfg.ServiceName)
	if err != nil {
		return nil, errors.New("failed to create context: " + err.Error())
	}

	clientConn := proto.NewClientConn(grpcservice)
	return clientConn, nil
}
