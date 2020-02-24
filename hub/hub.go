package hub

import (
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/hub/route"
)

// Parse call at the beginning of clash
func Parse(externalUI *string, externalController *string, secret *string) error {
	cfg, err := executor.Parse()
	if err != nil {
		return err
	}

	if externalUI != nil {
		cfg.General.ExternalUI = *externalUI
	}
	if externalController != nil {
		cfg.General.ExternalController = *externalController
	}
	if secret != nil {
		cfg.General.Secret = *secret
	}

	if cfg.General.ExternalUI != "" {
		route.SetUIPath(cfg.General.ExternalUI)
	}

	if cfg.General.ExternalController != "" {
		go route.Start(cfg.General.ExternalController, cfg.General.Secret)
	}

	executor.ApplyConfig(cfg, true)
	return nil
}
