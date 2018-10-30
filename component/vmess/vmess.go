package vmess

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"runtime"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
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
	AddrType byte
	Addr     []byte
	Port     uint
}

// Client is vmess connection generator
type Client struct {
	user           []*ID
	uuid           *uuid.UUID
	security       Security
	tls            bool
	host           string
	websocket      bool
	websocketPath  string
	skipCertVerify bool
}

// Config of vmess
type Config struct {
	UUID           string
	AlterID        uint16
	Security       string
	TLS            bool
	Host           string
	NetWork        string
	WebSocketPath  string
	SkipCertVerify bool
}

// New return a Conn with net.Conn and DstAddr
func (c *Client) New(conn net.Conn, dst *DstAddr) (net.Conn, error) {
	r := rand.Intn(len(c.user))
	if c.websocket {
		dialer := &websocket.Dialer{
			NetDial: func(network, addr string) (net.Conn, error) {
				return conn, nil
			},
			ReadBufferSize:   4 * 1024,
			WriteBufferSize:  4 * 1024,
			HandshakeTimeout: time.Second * 8,
		}
		scheme := "ws"
		if c.tls {
			scheme = "wss"
			dialer.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: c.skipCertVerify,
			}
		}

		host, port, err := net.SplitHostPort(c.host)
		if (scheme == "ws" && port != "80") || (scheme == "wss" && port != "443") {
			host = c.host
		}

		uri := url.URL{
			Scheme: scheme,
			Host:   host,
			Path:   c.websocketPath,
		}

		wsConn, resp, err := dialer.Dial(uri.String(), nil)
		if err != nil {
			var reason string
			if resp != nil {
				reason = resp.Status
			}
			return nil, fmt.Errorf("Dial %s error: %s", host, reason)
		}

		conn = newWebsocketConn(wsConn, conn.RemoteAddr())
	} else if c.tls {
		conn = tls.Client(conn, &tls.Config{
			InsecureSkipVerify: c.skipCertVerify,
		})
	}
	return newConn(conn, c.user[r], dst, c.security), nil
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

	return &Client{
		user:          newAlterIDs(newID(&uid), config.AlterID),
		uuid:          &uid,
		security:      security,
		tls:           config.TLS,
		host:          config.Host,
		websocket:     config.NetWork == "ws",
		websocketPath: config.WebSocketPath,
	}, nil
}
