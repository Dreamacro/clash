package main

import (
	"os"
	"os/signal"
	"syscall"
	"flag"
	"path"
	"path/filepath"

	"github.com/Dreamacro/clash/config"
	"github.com/Dreamacro/clash/hub"
	"github.com/Dreamacro/clash/proxy"
	"github.com/Dreamacro/clash/tunnel"
	C "github.com/Dreamacro/clash/constant"

	log "github.com/sirupsen/logrus"
)

var (
	homedir string
)

func init() {
	flag.StringVar(&homedir, "d", "", "set configuration directory")
	flag.Parse()
}

func main() {
	tunnel.Instance().Run()
	proxy.Instance().Run()
	hub.Run()

	if (homedir != "") {
		if !filepath.IsAbs(homedir) {
			currentDir, _ := os.Getwd()
			homedir = path.Join(currentDir, homedir)
		}
		C.SetHomeDir(homedir)
	}

	config.Init()
	err := config.Instance().Parse()
	if err != nil {
		log.Fatalf("Parse config error: %s", err.Error())
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
