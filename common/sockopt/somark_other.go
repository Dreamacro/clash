// +build !linux

package sockopt

import (
	"net"
)

func Mark(dialer *net.Dialer, mark int) {
	return
}
