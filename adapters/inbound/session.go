package adapters

import (
	"sync"
	"time"

	nat "github.com/Dreamacro/clash/component/nat-table"
)

var (
	natTable *nat.Table
	once     sync.Once

	natTimeout = 120 * time.Second
)

func NATInstance() *nat.Table {
	once.Do(func() {
		natTable = nat.New(natTimeout)
	})
	return natTable
}
