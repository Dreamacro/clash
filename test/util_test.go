package main

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/net/route"
)

func defaultRouteIP() (net.IP, error) {
	idx, err := defaultRouteInterfaceIndex()
	if err != nil {
		return nil, err
	}
	iface, err := net.InterfaceByIndex(idx)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ip := addr.(*net.IPNet).IP
		if ip.To4() != nil {
			return ip, nil
		}
	}

	return nil, errors.New("no ipv4 addr")
}

func defaultRouteInterfaceIndex() (int, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, syscall.NET_RT_DUMP2, 0)
	if err != nil {
		return 0, fmt.Errorf("route.FetchRIB: %w", err)
	}
	msgs, err := route.ParseRIB(syscall.NET_RT_IFLIST2, rib)
	if err != nil {
		return 0, fmt.Errorf("route.ParseRIB: %w", err)
	}
	for _, message := range msgs {
		routeMessage := message.(*route.RouteMessage)
		if routeMessage.Flags&(syscall.RTF_UP|syscall.RTF_GATEWAY|syscall.RTF_STATIC) == 0 {
			continue
		}

		addresses := routeMessage.Addrs

		destination, ok := addresses[0].(*route.Inet4Addr)
		if !ok {
			continue
		}

		if destination.IP != [4]byte{0, 0, 0, 0} {
			continue
		}

		switch addresses[1].(type) {
		case *route.Inet4Addr:
			return routeMessage.Index, nil
		default:
			continue
		}
	}

	return 0, fmt.Errorf("ambiguous gateway interfaces found")
}
