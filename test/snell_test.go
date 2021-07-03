package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/Dreamacro/clash/adapter/outbound"
	C "github.com/Dreamacro/clash/constant"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestClash_SnellObfsHTTP(t *testing.T) {
	cfg := &container.Config{
		Image:        ImageSnell,
		ExposedPorts: defaultExposedPorts,
		Cmd:          []string{"-c", "/config.conf"},
	}
	hostCfg := &container.HostConfig{
		PortBindings: defaultPortBindings,
		Binds:        []string{fmt.Sprintf("%s:/config.conf", C.Path.Resolve("snell-http.conf"))},
	}

	id, err := startContainer(cfg, hostCfg, "snell-http")
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	t.Cleanup(func() {
		cleanContainer(id)
	})

	proxy, err := outbound.NewSnell(outbound.SnellOption{
		Name:   "snell",
		Server: localIP.String(),
		Port:   10002,
		Psk:    "password",
		ObfsOpts: map[string]interface{}{
			"mode": "http",
		},
	})
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	time.Sleep(waitTime)
	testSuit(t, proxy)
}

func TestClash_SnellObfsTLS(t *testing.T) {
	cfg := &container.Config{
		Image:        ImageSnell,
		ExposedPorts: defaultExposedPorts,
		Cmd:          []string{"-c", "/config.conf"},
	}
	hostCfg := &container.HostConfig{
		PortBindings: defaultPortBindings,
		Binds:        []string{fmt.Sprintf("%s:/config.conf", C.Path.Resolve("snell-tls.conf"))},
	}

	id, err := startContainer(cfg, hostCfg, "snell-tls")
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	t.Cleanup(func() {
		cleanContainer(id)
	})

	proxy, err := outbound.NewSnell(outbound.SnellOption{
		Name:   "snell",
		Server: localIP.String(),
		Port:   10002,
		Psk:    "password",
		ObfsOpts: map[string]interface{}{
			"mode": "tls",
		},
	})
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	time.Sleep(waitTime)
	testSuit(t, proxy)
}

func TestClash_Snell(t *testing.T) {
	cfg := &container.Config{
		Image:        ImageSnell,
		ExposedPorts: defaultExposedPorts,
		Cmd:          []string{"-c", "/config.conf"},
	}
	hostCfg := &container.HostConfig{
		PortBindings: defaultPortBindings,
		Binds:        []string{fmt.Sprintf("%s:/config.conf", C.Path.Resolve("snell.conf"))},
	}

	id, err := startContainer(cfg, hostCfg, "snell")
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	t.Cleanup(func() {
		cleanContainer(id)
	})

	proxy, err := outbound.NewSnell(outbound.SnellOption{
		Name:   "snell",
		Server: localIP.String(),
		Port:   10002,
		Psk:    "password",
	})
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	time.Sleep(waitTime)
	testSuit(t, proxy)
}
