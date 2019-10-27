package tunnel

import (
	"net"
	"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/gofrs/uuid"
)

type tracker interface {
	ID() string
	Close() error
}

type trackerInfo struct {
	UUID          uuid.UUID   `json:"id"`
	Metadata      *C.Metadata `json:"metadata"`
	UploadTotal   int64       `json:"upload"`
	DownloadTotal int64       `json:"download"`
	Start         time.Time   `json:"start"`
	Chain         C.Chain     `json:"chains"`
	Rule          string      `json:"rule"`
}

type tcpTracker struct {
	C.Conn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (tt *tcpTracker) ID() string {
	return tt.UUID.String()
}

func (tt *tcpTracker) Read(b []byte) (int, error) {
	n, err := tt.Conn.Read(b)
	download := int64(n)
	tt.manager.Download() <- download
	tt.DownloadTotal += download
	return n, err
}

func (tt *tcpTracker) Write(b []byte) (int, error) {
	n, err := tt.Conn.Write(b)
	upload := int64(n)
	tt.manager.Upload() <- upload
	tt.UploadTotal += upload
	return n, err
}

func (tt *tcpTracker) Close() error {
	tt.manager.Leave(tt)
	return tt.Conn.Close()
}

func newTCPTracker(conn C.Conn, manager *Manager, metadata *C.Metadata, rule C.Rule) *tcpTracker {
	uuid, _ := uuid.NewV4()
	t := &tcpTracker{
		Conn:    conn,
		manager: manager,
		trackerInfo: &trackerInfo{
			UUID:     uuid,
			Start:    time.Now(),
			Metadata: metadata,
			Chain:    conn.Chains(),
			Rule:     rule.RuleType().String(),
		},
	}

	manager.Join(t)
	return t
}

type udpTracker struct {
	C.PacketConn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (ut *udpTracker) ID() string {
	return ut.UUID.String()
}

func (ut *udpTracker) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := ut.PacketConn.ReadFrom(b)
	download := int64(n)
	ut.manager.Download() <- download
	ut.DownloadTotal += download
	return n, addr, err
}

func (ut *udpTracker) WriteTo(b []byte, addr net.Addr) (int, error) {
	n, err := ut.PacketConn.WriteTo(b, addr)
	upload := int64(n)
	ut.manager.Upload() <- upload
	ut.UploadTotal += upload
	return n, err
}

func (ut *udpTracker) Close() error {
	ut.manager.Leave(ut)
	return ut.PacketConn.Close()
}

func newUDPTracker(conn C.PacketConn, manager *Manager, metadata *C.Metadata, rule C.Rule) *udpTracker {
	uuid, _ := uuid.NewV4()
	ut := &udpTracker{
		PacketConn: conn,
		manager:    manager,
		trackerInfo: &trackerInfo{
			UUID:     uuid,
			Start:    time.Now(),
			Metadata: metadata,
			Chain:    conn.Chains(),
			Rule:     rule.RuleType().String(),
		},
	}

	manager.Join(ut)
	return ut
}
