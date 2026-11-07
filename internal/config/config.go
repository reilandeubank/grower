// Package config reads grower's settings from GROWER_* environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Cookie names. The __Host- prefix makes browsers insist on Secure, Path=/ and no
// Domain, so it is only usable over https.
const (
	SecureSessionCookie   = "__Host-grower_session"
	InsecureSessionCookie = "grower_session"
)

// Config is the validated runtime configuration.
type Config struct {
	DataDir string
	Listen  string
	// BaseURL is the public origin users reach grower on. It is authoritative for
	// cookie security and the CSRF Origin check. Nil when GROWER_BASE_URL is unset.
	BaseURL        *url.URL
	TrustedProxies []netip.Prefix
	// Location is the time zone given to newly created users.
	Location  *time.Location
	LogFormat string // "json" or "text"
	LogLevel  slog.Level
	// Species data providers (M8). Each is optional; empty disables it. These are
	// secrets: never log them.
	OpenPlantbookClientID     string
	OpenPlantbookClientSecret string
	PerenualAPIKey            string
	// Pl@ntNet plant identification (M9), also optional.
	PlantNetAPIKey string
	// MQTT is the Home Assistant MQTT publisher's broker; nil when GROWER_MQTT_URL is unset.
	MQTT *MQTT
}

// MQTT is where the Home Assistant publisher connects, and the topics it uses.
type MQTT struct {
	// Broker is the paho server URL (tcp://, ssl://, ws:// or wss://) with a port and
	// without credentials, so it is safe to log.
	Broker *url.URL
	// Username and Password are secrets: never log Password.
	Username, Password string
	// Topic prefixes grower's own topics; DiscoveryPrefix is Home Assistant's.
	Topic, DiscoveryPrefix string
}

// Load parses the environment through getenv (os.Getenv in production). It returns
// every problem found, not just the first. GROWER_BASE_URL is optional here; commands
// that need it call RequireBaseURL.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DataDir:   orDefault(getenv("GROWER_DATA_DIR"), "/data"),
		Listen:    orDefault(getenv("GROWER_LISTEN"), ":8080"),
		LogFormat: orDefault(getenv("GROWER_LOG_FORMAT"), "json"),

		OpenPlantbookClientID:     strings.TrimSpace(getenv("GROWER_OPENPLANTBOOK_CLIENT_ID")),
		OpenPlantbookClientSecret: strings.TrimSpace(getenv("GROWER_OPENPLANTBOOK_CLIENT_SECRET")),
		PerenualAPIKey:            strings.TrimSpace(getenv("GROWER_PERENUAL_API_KEY")),
		PlantNetAPIKey:            strings.TrimSpace(getenv("GROWER_PLANTNET_API_KEY")),
	}
	var errs []error

	if _, err := ListenPort(cfg.Listen); err != nil {
		errs = append(errs, fmt.Errorf("GROWER_LISTEN: %w", err))
	}

	if raw := getenv("GROWER_BASE_URL"); raw != "" {
		u, err := parseBaseURL(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("GROWER_BASE_URL: %w", err))
		}
		cfg.BaseURL = u
	}

	proxies, err := parseProxies(getenv("GROWER_TRUSTED_PROXIES"))
	if err != nil {
		errs = append(errs, fmt.Errorf("GROWER_TRUSTED_PROXIES: %w", err))
	}
	cfg.TrustedProxies = proxies

	loc, err := time.LoadLocation(orDefault(getenv("GROWER_TZ"), "UTC"))
	if err != nil {
		errs = append(errs, fmt.Errorf("GROWER_TZ: unknown IANA time zone: %w", err))
	}
	cfg.Location = loc

	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("GROWER_LOG_FORMAT: must be json or text, got %q", cfg.LogFormat))
	}
	if raw := getenv("GROWER_LOG_LEVEL"); raw != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(raw)); err != nil {
			errs = append(errs, fmt.Errorf("GROWER_LOG_LEVEL: must be debug, info, warn or error, got %q", raw))
		}
	}

	if raw := strings.TrimSpace(getenv("GROWER_MQTT_URL")); raw != "" {
		m, err := parseMQTT(raw, getenv)
		if err != nil {
			errs = append(errs, err)
		}
		cfg.MQTT = m
	}

	if (cfg.OpenPlantbookClientID == "") != (cfg.OpenPlantbookClientSecret == "") {
		errs = append(errs, errors.New("GROWER_OPENPLANTBOOK_CLIENT_ID and GROWER_OPENPLANTBOOK_CLIENT_SECRET must be set together"))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// RequireBaseURL reports an error when GROWER_BASE_URL was not set.
func (c Config) RequireBaseURL() error {
	if c.BaseURL == nil {
		return errors.New("GROWER_BASE_URL is required (the public URL, e.g. https://plants.example.com)")
	}
	return nil
}

// SecureCookies is true when the app is served over https.
func (c Config) SecureCookies() bool {
	return c.BaseURL != nil && c.BaseURL.Scheme == "https"
}

// SessionCookieName depends on the scheme because __Host- cookies require https.
func (c Config) SessionCookieName() string {
	if c.SecureCookies() {
		return SecureSessionCookie
	}
	return InsecureSessionCookie
}

// Origin is the scheme://host[:port] browsers send in the Origin header.
func (c Config) Origin() string {
	if c.BaseURL == nil {
		return ""
	}
	return c.BaseURL.Scheme + "://" + c.BaseURL.Host
}

// ListenPort extracts the port from a listen address such as ":8080" or "0.0.0.0:80".
func ListenPort(listen string) (int, error) {
	_, portStr, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, fmt.Errorf("must be host:port or :port, got %q", listen)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port in %q", listen)
	}
	return port, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("not a URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("must be an absolute http(s) URL, got %q", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("missing host in %q", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("must not contain credentials, a query or a fragment: %q", raw)
	}
	// The SPA and the __Host- cookie both assume the app owns the whole origin.
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("must not have a path (grower is served at the root of its host): %q", raw)
	}
	u.Path = ""
	u.Host = strings.ToLower(u.Host)
	return u, nil
}

func parseProxies(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Accept a bare IP as a single-host prefix.
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return nil, fmt.Errorf("invalid IP or CIDR %q", part)
			}
			out = append(out, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", part)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// mqttSchemes maps the schemes GROWER_MQTT_URL takes to paho's, with their default ports.
var mqttSchemes = map[string]struct {
	paho, port string
}{
	"mqtt": {"tcp", "1883"}, "tcp": {"tcp", "1883"},
	"mqtts": {"ssl", "8883"}, "ssl": {"ssl", "8883"},
	"ws": {"ws", "80"}, "wss": {"wss", "443"},
}

// parseMQTT reads GROWER_MQTT_URL and the settings that go with it. Credentials may be
// in the URL or in GROWER_MQTT_USERNAME and GROWER_MQTT_PASSWORD, which win.
func parseMQTT(raw string, getenv func(string) string) (*MQTT, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("GROWER_MQTT_URL: %w", redactURLError(err))
	}
	scheme, ok := mqttSchemes[strings.ToLower(u.Scheme)]
	if !ok || u.Hostname() == "" {
		return nil, errors.New("GROWER_MQTT_URL: must look like mqtt://host:1883, mqtts://host:8883, ws://host/mqtt or wss://host/mqtt")
	}
	m := &MQTT{
		Topic:           strings.Trim(orDefault(strings.TrimSpace(getenv("GROWER_MQTT_TOPIC")), "grower"), "/"),
		DiscoveryPrefix: strings.Trim(orDefault(strings.TrimSpace(getenv("GROWER_MQTT_DISCOVERY_PREFIX")), "homeassistant"), "/"),
		Username:        orDefault(getenv("GROWER_MQTT_USERNAME"), u.User.Username()),
		Password:        getenv("GROWER_MQTT_PASSWORD"),
	}
	if pw, set := u.User.Password(); set && m.Password == "" {
		m.Password = pw
	}
	port := u.Port()
	if port == "" {
		port = scheme.port
	}
	m.Broker = &url.URL{Scheme: scheme.paho, Host: net.JoinHostPort(u.Hostname(), port), Path: u.Path}
	var errs []error
	for _, t := range []struct{ name, topic string }{{"GROWER_MQTT_TOPIC", m.Topic}, {"GROWER_MQTT_DISCOVERY_PREFIX", m.DiscoveryPrefix}} {
		name, topic := t.name, t.topic
		if topic == "" || strings.ContainsAny(topic, "+#\x00") {
			errs = append(errs, fmt.Errorf("%s: must be a topic without + or #, got %q", name, topic))
		}
	}
	return m, errors.Join(errs...)
}

// redactURLError drops the URL from a parse error, since it may hold a password.
func redactURLError(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return uerr.Err
	}
	return err
}
