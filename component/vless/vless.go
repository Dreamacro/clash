package vless

import (
	"net"

	"github.com/Dreamacro/clash/component/vmess"
	"github.com/gofrs/uuid"
)

const Version byte = 0 // protocol version. preview version is 0

// Client is vless connection generator
type Client struct {
	uuid *uuid.UUID
}

// StreamConn return a Conn with net.Conn and DstAddr
func (c *Client) StreamConn(conn net.Conn, dst *vmess.DstAddr) (net.Conn, error) {
	return newConn(conn, c.uuid, dst)
}

// NewClient return Client instance
func NewClient(uuidStr string) (*Client, error) {
	uid, err := uuid.FromString(uuidStr)
	if err != nil {
		return nil, err
	}

	return &Client{
		uuid: &uid,
	}, nil
}
