package mobile

import (
	"os"
  "context"
	"path/filepath"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub/executor"

	log "github.com/sirupsen/logrus"
)

var (
  ctx context.Context
  cancel context.CancelFunc
)


func Start(homedir string) {
	os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",tls13=1")

	if homedir != "" {
		if !filepath.IsAbs(homedir) {
			currentDir, _ := os.Getwd()
			homedir = filepath.Join(currentDir, homedir)
		}
		C.SetHomeDir(homedir)
	}

	if err := config.Init(C.Path.HomeDir()); err != nil {
		log.Fatalf("Initial configuration directory error: %s", err.Error())
	}

  cfg, err := executor.Parse()
  if err != nil {
    return
  }
  ctx, cancel = context.WithCancel(context.Background())
  executor.ApplyConfig(ctx, cfg, true)
  return
}

func IsRunning()(bool){
  return bool(ctx != nil)
}

func Stop(){
  cancel()
  ctx = nil
}
