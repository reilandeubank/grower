package main

import (
	"fmt"
	"log/slog"

	"github.com/reilandeubank/grower/internal/config"
)

// logSetup tells the operator how to create the first account. A generated code is
// printed in the message itself, at warn level, so it stands out in a container log
// viewer; a code from GROWER_SETUP_CODE is a secret the operator already has, so it
// is never logged.
func logSetup(log *slog.Logger, cfg config.Config, code string) {
	url := cfg.BaseURL.String()
	if cfg.SetupCode != "" {
		log.Warn(fmt.Sprintf("No account yet. Open %s and enter the setup code from GROWER_SETUP_CODE.", url))
		return
	}
	log.Warn(fmt.Sprintf("No account yet. Open %s and enter setup code %s", url, code), "setup_code", code)
}
