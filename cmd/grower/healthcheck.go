package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/reilandeubank/grower/internal/config"
)

// cmdHealthcheck exists because the distroless image has no curl or wget. It only
// reads GROWER_LISTEN so an unrelated config problem can't mask the server's health.
func cmdHealthcheck(args []string) error {
	flags := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	listen := os.Getenv("GROWER_LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	port, err := config.ListenPort(listen)
	if err != nil {
		return fmt.Errorf("GROWER_LISTEN: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // G704: fixed loopback URL, validated port
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: see above
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s returned %s", url, resp.Status)
	}
	return nil
}
