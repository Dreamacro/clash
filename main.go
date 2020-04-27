package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/Dreamacro/clash/config"
	"github.com/Dreamacro/clash/constant"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/log"
)

var (
	flagset            map[string]bool
	version            bool
	testConfig         bool
	homeDir            string
	configFile         string
	externalUI         string
	externalController string
	secret             string
)

func init() {
	flag.StringVar(&homeDir, "d", "", "set configuration directory")
	flag.StringVar(&configFile, "f", "", "specify configuration file")
	flag.StringVar(&externalUI, "ext-ui", "", "override external ui directory")
	flag.StringVar(&externalController, "ext-ctl", "", "override external controller address")
	flag.StringVar(&secret, "secret", "", "override secret for RESTful API")
	flag.BoolVar(&version, "v", false, "show current version of clash")
	flag.BoolVar(&testConfig, "t", false, "test configuration and exit")
	flag.Parse()

	flagset = map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		flagset[f.Name] = true
	})
}

func main() {
	if version {
		fmt.Printf("Clash %s %s %s %s\n", C.Version, runtime.GOOS, runtime.GOARCH, C.BuildTime)
		return
	}

	if homeDir != "" {
		if !filepath.IsAbs(homeDir) {
			currentDir, _ := os.Getwd()
			homeDir = filepath.Join(currentDir, homeDir)
		}
		C.SetHomeDir(homeDir)
	}

	if configFile != "" {
		if !filepath.IsAbs(configFile) {
			currentDir, _ := os.Getwd()
			configFile = filepath.Join(currentDir, configFile)
		}
		C.SetConfig(configFile)
	} else {
		configFile := filepath.Join(C.Path.HomeDir(), C.Path.Config())
		C.SetConfig(configFile)
	}

	if err := config.Init(C.Path.HomeDir()); err != nil {
		log.Fatalln("Initial configuration directory error: %s", err.Error())
	}

	if testConfig {
		if _, err := executor.Parse(); err != nil {
			log.Errorln(err.Error())
			fmt.Printf("configuration file %s test failed\n", constant.Path.Config())
			os.Exit(1)
		}
		fmt.Printf("configuration file %s test is successful\n", constant.Path.Config())
		return
	}

	var options []hub.Option
	if flagset["ext-ui"] {
		options = append(options, hub.WithExternalUI(externalUI))
	}
	if flagset["ext-ctl"] {
		options = append(options, hub.WithExternalController(externalController))
	}
	if flagset["secret"] {
		options = append(options, hub.WithSecret(secret))
	}

	if err := hub.Parse(options...); err != nil {
		log.Fatalln("Parse config error: %s", err.Error())
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
