package main

import (
	"bytes"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/config"
)

func TestLogSetup(t *testing.T) {
	base, _ := url.Parse("https://plants.example.com")
	tests := []struct {
		name     string
		setup    string // GROWER_SETUP_CODE
		code     string
		want     []string
		wantNone []string
	}{
		{name: "generated", code: "K7Q2M-X9PB4", want: []string{"https://plants.example.com", "K7Q2M-X9PB4"}},
		{name: "from env", setup: "our-secret-code", code: "our-secret-code", want: []string{"GROWER_SETUP_CODE"}, wantNone: []string{"our-secret-code"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, nil))
			logSetup(log, config.Config{BaseURL: base, SetupCode: tt.setup}, tt.code)
			out := buf.String()
			for _, s := range tt.want {
				if !strings.Contains(out, s) {
					t.Errorf("log %q does not contain %q", out, s)
				}
			}
			for _, s := range tt.wantNone {
				if strings.Contains(out, s) {
					t.Errorf("log %q leaks %q", out, s)
				}
			}
		})
	}
}
