package tunnel

import (
	"sync"
	"time"

	nat "github.com/Dreamacro/clash/component/nat-table"
)

var (
	natOnce  sync.Once
	natPool  *nat.Pool
	natTable *nat.Table

	udpTimeout = 120 * time.Second
)

func NATInstance() *nat.Table {
	natOnce.Do(func() {
		natPool = nat.NewPool()
		natTable = nat.NewTable()
	})
	return natTable
}
