package context

import (
	"net"

	C "github.com/Dreamacro/clash/constant"

	"github.com/gofrs/uuid"
)

type PacketConnContext struct {
	id         uuid.UUID
	metadata   *C.Metadata
	packetConn net.PacketConn
}

func NewPacketConnContext(metadata *C.Metadata) *PacketConnContext {
	id, _ := uuid.NewV4()
	return &PacketConnContext{
		id:       id,
		metadata: metadata,
	}
}

// ID implement C.PacketConnContext ID
func (pc *PacketConnContext) ID() uuid.UUID {
	return pc.id
}

// Metadata implement C.PacketConnContext Metadata
func (pc *PacketConnContext) Metadata() *C.Metadata {
	return pc.metadata
}

// PacketConn implement C.PacketConnContext PacketConn
func (pc *PacketConnContext) PacketConn() net.PacketConn {
	return pc.packetConn
}

// InjectPacketConn injectPacketConn manually
func (pc *PacketConnContext) InjectPacketConn(pconn net.PacketConn) {
	pc.packetConn = pconn
}
