package config

import (
	"log/slog"
	"strings"
	"testing"
)

func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envFunc(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != "/data" || cfg.Listen != ":8080" || cfg.LogFormat != "json" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.Location.String() != "UTC" || cfg.LogLevel != slog.LevelInfo {
		t.Errorf("unexpected tz/level: %v %v", cfg.Location, cfg.LogLevel)
	}
	if cfg.BaseURL != nil || len(cfg.TrustedProxies) != 0 {
		t.Errorf("expected no base URL or proxies")
	}
	if err := cfg.RequireBaseURL(); err == nil {
		t.Error("RequireBaseURL should fail without GROWER_BASE_URL")
	}
}

func TestLoadBaseURLAndCookies(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr bool
		origin  string
		cookie  string
		secure  bool
	}{
		{raw: "https://Plants.Example.com", origin: "https://plants.example.com", cookie: SecureSessionCookie, secure: true},
		{raw: "https://plants.example.com/", origin: "https://plants.example.com", cookie: SecureSessionCookie, secure: true},
		{raw: "http://localhost:5173", origin: "http://localhost:5173", cookie: InsecureSessionCookie},
		{raw: "ftp://example.com", wantErr: true},
		{raw: "plants.example.com", wantErr: true},
		{raw: "https://example.com/grower", wantErr: true},
		{raw: "https://example.com/?x=1", wantErr: true},
		{raw: "https://user:pw@example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			cfg, err := Load(envFunc(map[string]string{"GROWER_BASE_URL": tt.raw}))
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "GROWER_BASE_URL") {
					t.Fatalf("want GROWER_BASE_URL error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.Origin(); got != tt.origin {
				t.Errorf("Origin = %q, want %q", got, tt.origin)
			}
			if got := cfg.SessionCookieName(); got != tt.cookie {
				t.Errorf("cookie = %q, want %q", got, tt.cookie)
			}
			if cfg.SecureCookies() != tt.secure {
				t.Errorf("SecureCookies = %v", cfg.SecureCookies())
			}
		})
	}
}

func TestLoadValues(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, c Config)
	}{
		{
			name: "proxies",
			env:  map[string]string{"GROWER_TRUSTED_PROXIES": " 172.18.0.0/16, 10.0.0.1 ,fd00::/8"},
			check: func(t *testing.T, c Config) {
				got := []string{}
				for _, p := range c.TrustedProxies {
					got = append(got, p.String())
				}
				if strings.Join(got, ",") != "172.18.0.0/16,10.0.0.1/32,fd00::/8" {
					t.Errorf("proxies = %v", got)
				}
			},
		},
		{name: "bad proxy", env: map[string]string{"GROWER_TRUSTED_PROXIES": "nope"}, wantErr: "GROWER_TRUSTED_PROXIES"},
		{
			name: "timezone",
			env:  map[string]string{"GROWER_TZ": "America/Chicago"},
			check: func(t *testing.T, c Config) {
				if c.Location.String() != "America/Chicago" {
					t.Errorf("tz = %v", c.Location)
				}
			},
		},
		{name: "bad timezone", env: map[string]string{"GROWER_TZ": "Mars/Base"}, wantErr: "GROWER_TZ"},
		{name: "bad log format", env: map[string]string{"GROWER_LOG_FORMAT": "xml"}, wantErr: "GROWER_LOG_FORMAT"},
		{
			name: "log level",
			env:  map[string]string{"GROWER_LOG_LEVEL": "debug", "GROWER_LOG_FORMAT": "text"},
			check: func(t *testing.T, c Config) {
				if c.LogLevel != slog.LevelDebug || c.LogFormat != "text" {
					t.Errorf("level=%v format=%v", c.LogLevel, c.LogFormat)
				}
			},
		},
		{name: "bad log level", env: map[string]string{"GROWER_LOG_LEVEL": "loud"}, wantErr: "GROWER_LOG_LEVEL"},
		{name: "bad listen", env: map[string]string{"GROWER_LISTEN": "8080"}, wantErr: "GROWER_LISTEN"},
		{name: "bad port", env: map[string]string{"GROWER_LISTEN": ":99999"}, wantErr: "GROWER_LISTEN"},
		{
			name: "species providers",
			env: map[string]string{
				"GROWER_OPENPLANTBOOK_CLIENT_ID": " id ", "GROWER_OPENPLANTBOOK_CLIENT_SECRET": "secret", "GROWER_PERENUAL_API_KEY": "key",
			},
			check: func(t *testing.T, c Config) {
				if c.OpenPlantbookClientID != "id" || c.OpenPlantbookClientSecret != "secret" || c.PerenualAPIKey != "key" {
					t.Errorf("providers = %q %q %q", c.OpenPlantbookClientID, c.OpenPlantbookClientSecret, c.PerenualAPIKey)
				}
			},
		},
		{name: "half of plantbook", env: map[string]string{"GROWER_OPENPLANTBOOK_CLIENT_ID": "id"}, wantErr: "GROWER_OPENPLANTBOOK_CLIENT_SECRET"},
		{
			name:    "reports all errors",
			env:     map[string]string{"GROWER_TZ": "x", "GROWER_LOG_FORMAT": "y"},
			wantErr: "GROWER_LOG_FORMAT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(envFunc(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestListenPort(t *testing.T) {
	for in, want := range map[string]int{":8080": 8080, "0.0.0.0:80": 80, "[::]:9000": 9000} {
		got, err := ListenPort(in)
		if err != nil || got != want {
			t.Errorf("ListenPort(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}
