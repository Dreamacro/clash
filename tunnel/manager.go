package tunnel

import (
	"sync"
	"time"
)

var DefaultManager *Manager

func init() {
	DefaultManager = &Manager{
		upload:   make(chan int64),
		download: make(chan int64),
	}
	DefaultManager.handle()
}

type Manager struct {
	connections   sync.Map
	upload        chan int64
	download      chan int64
	uploadTemp    int64
	downloadTemp  int64
	uploadBlip    int64
	downloadBlip  int64
	uploadTotal   int64
	downloadTotal int64
}

func (m *Manager) Join(c tracker) {
	m.connections.Store(c.ID(), c)
}

func (m *Manager) Leave(c tracker) {
	m.connections.Delete(c.ID())
}

func (m *Manager) Upload() chan<- int64 {
	return m.upload
}

func (m *Manager) Download() chan<- int64 {
	return m.download
}

func (m *Manager) Now() (up int64, down int64) {
	return m.uploadBlip, m.downloadBlip
}

func (m *Manager) Snapshot() *Snapshot {
	connections := []tracker{}
	m.connections.Range(func(key, value interface{}) bool {
		connections = append(connections, value.(tracker))
		return true
	})

	return &Snapshot{
		UploadTotal:   m.uploadTotal,
		DownloadTotal: m.downloadTotal,
		Connections:   connections,
	}
}

func (m *Manager) handle() {
	go m.handleCh(m.upload, &m.uploadTemp, &m.uploadBlip, &m.uploadTotal)
	go m.handleCh(m.download, &m.downloadTemp, &m.downloadBlip, &m.downloadTotal)
}

func (m *Manager) handleCh(ch <-chan int64, temp *int64, blip *int64, total *int64) {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case n := <-ch:
			*temp += n
			*total += n
		case <-ticker.C:
			*blip = *temp
			*temp = 0
		}
	}
}

type Snapshot struct {
	DownloadTotal int64     `json:"downloadTotal"`
	UploadTotal   int64     `json:"uploadTotal"`
	Connections   []tracker `json:"connections"`
}
