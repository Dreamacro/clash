package vmess

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"sync"

	"github.com/gofrs/uuid"
)

// Version of vmess
const Version byte = 1

// Request Options
const (
	OptionChunkStream  byte = 1
	OptionChunkMasking byte = 4
)

// Security type vmess
type Security = byte

// Cipher types
const (
	SecurityAES128GCM        Security = 3
	SecurityCHACHA20POLY1305 Security = 4
	SecurityNone             Security = 5
)

// CipherMapping return
var CipherMapping = map[string]byte{
	"none":              SecurityNone,
	"aes-128-gcm":       SecurityAES128GCM,
	"chacha20-poly1305": SecurityCHACHA20POLY1305,
}

var (
	clientSessionCache tls.ClientSessionCache
	once               sync.Once
)

// Command types
const (
	CommandTCP byte = 1
	CommandUDP byte = 2
)

// Addr types
const (
	AtypIPv4       byte = 1
	AtypDomainName byte = 2
	AtypIPv6       byte = 3
)

// DstAddr store destination address
type DstAddr struct {
	UDP      bool
	AddrType byte
	Addr     []byte
	Port     uint
}

// Client is vmess connection generator
type Client struct {
	user      []*ID
	uuid      *uuid.UUID
	security  Security
	tls       bool
	host      string
	wsConfig  *WebsocketConfig
	tlsConfig *tls.Config
}

// Config of vmess
type Config struct {
	UUID             string
	AlterID          uint16
	Security         string
	TLS              bool
	HostName         string
	Port             string
	NetWork          string
	WebSocketPath    string
	WebSocketHeaders map[string]string
	SkipCertVerify   bool
	SessionCache     tls.ClientSessionCache
}

// New return a Conn with net.Conn and DstAddr
func (c *Client) New(conn net.Conn, dst *DstAddr) (net.Conn, error) {
	var err error
	r := rand.Intn(len(c.user))
	if c.wsConfig != nil {
		conn, err = NewWebsocketConn(conn, c.wsConfig)
		if err != nil {
			return nil, err
		}
	} else if c.tls {
		conn = tls.Client(conn, c.tlsConfig)
	}
	return newConn(conn, c.user[r], dst, c.security)
}

// NewClient return Client instance
func NewClient(config Config) (*Client, error) {
	uid, err := uuid.FromString(config.UUID)
	if err != nil {
		return nil, err
	}

	var security Security
	switch config.Security {
	case "aes-128-gcm":
		security = SecurityAES128GCM
	case "chacha20-poly1305":
		security = SecurityCHACHA20POLY1305
	case "none":
		security = SecurityNone
	case "auto":
		security = SecurityCHACHA20POLY1305
		if runtime.GOARCH == "amd64" || runtime.GOARCH == "s390x" || runtime.GOARCH == "arm64" {
			security = SecurityAES128GCM
		}
	default:
		return nil, fmt.Errorf("Unknown security type: %s", config.Security)
	}

	if config.NetWork != "" && config.NetWork != "ws" {
		return nil, fmt.Errorf("Unknown network type: %s", config.NetWork)
	}

	header := http.Header{}
	for k, v := range config.WebSocketHeaders {
		header.Add(k, v)
	}

	host := net.JoinHostPort(config.HostName, config.Port)

	var tlsConfig *tls.Config
	if config.TLS {
		tlsConfig = &tls.Config{
			ServerName:         config.HostName,
			InsecureSkipVerify: config.SkipCertVerify,
			ClientSessionCache: config.SessionCache,
		}
		if tlsConfig.ClientSessionCache == nil {
			tlsConfig.ClientSessionCache = getClientSessionCache()
		}
		if host := header.Get("Host"); host != "" {
			tlsConfig.ServerName = host
		}
	}

	var wsConfig *WebsocketConfig
	if config.NetWork == "ws" {
		wsConfig = &WebsocketConfig{
			Host:      host,
			Path:      config.WebSocketPath,
			Headers:   header,
			TLS:       config.TLS,
			TLSConfig: tlsConfig,
		}
	}

	return &Client{
		user:      newAlterIDs(newID(&uid), config.AlterID),
		uuid:      &uid,
		security:  security,
		tls:       config.TLS,
		host:      host,
		wsConfig:  wsConfig,
		tlsConfig: tlsConfig,
	}, nil
}

func getClientSessionCache() tls.ClientSessionCache {
	once.Do(func() {
		clientSessionCache = tls.NewLRUClientSessionCache(128)
	})
	return clientSessionCache
}
