// +build !linux,!darwin

package sockopt

import (
	"net"
)

func BindToDevice(dialer *net.Dialer, ifname string) {
	// TODO: Implement for other OSs
	return
}
