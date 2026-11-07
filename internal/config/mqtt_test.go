package config

import (
	"strings"
	"testing"
)

func TestLoadMQTT(t *testing.T) {
	if cfg, err := Load(envFunc(nil)); err != nil || cfg.MQTT != nil {
		t.Fatalf("unset: MQTT = %+v, err %v", cfg.MQTT, err)
	}
	tests := []struct {
		name                     string
		env                      map[string]string
		broker, user, pass       string
		topic, discovery, errSub string
	}{
		{name: "defaults", env: map[string]string{"GROWER_MQTT_URL": "mqtt://broker.lan"}, broker: "tcp://broker.lan:1883", topic: "grower", discovery: "homeassistant"},
		{name: "tls and creds in url", env: map[string]string{"GROWER_MQTT_URL": "mqtts://ha:p%40ss@broker.lan"}, broker: "ssl://broker.lan:8883", user: "ha", pass: "p@ss", topic: "grower", discovery: "homeassistant"},
		{
			name: "separate creds win, custom topics",
			env: map[string]string{
				"GROWER_MQTT_URL": "mqtt://old:old@10.0.0.2:1884", "GROWER_MQTT_USERNAME": "grower", "GROWER_MQTT_PASSWORD": "s3cret",
				"GROWER_MQTT_TOPIC": "/home/plants/", "GROWER_MQTT_DISCOVERY_PREFIX": "ha",
			},
			broker: "tcp://10.0.0.2:1884", user: "grower", pass: "s3cret", topic: "home/plants", discovery: "ha",
		},
		{name: "websocket", env: map[string]string{"GROWER_MQTT_URL": "wss://broker.example/mqtt"}, broker: "wss://broker.example:443/mqtt", topic: "grower", discovery: "homeassistant"},
		{name: "bad scheme", env: map[string]string{"GROWER_MQTT_URL": "http://broker"}, errSub: "GROWER_MQTT_URL"},
		{name: "no host", env: map[string]string{"GROWER_MQTT_URL": "broker:1883"}, errSub: "GROWER_MQTT_URL"},
		{name: "wildcard topic", env: map[string]string{"GROWER_MQTT_URL": "mqtt://b", "GROWER_MQTT_TOPIC": "grower/#"}, errSub: "GROWER_MQTT_TOPIC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(envFunc(tt.env))
			if tt.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("err = %v, want it to mention %s", err, tt.errSub)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			m := cfg.MQTT
			if m.Broker.String() != tt.broker || m.Username != tt.user || m.Password != tt.pass || m.Topic != tt.topic || m.DiscoveryPrefix != tt.discovery {
				t.Errorf("got broker %s user %q pass %q topic %q discovery %q", m.Broker, m.Username, m.Password, m.Topic, m.DiscoveryPrefix)
			}
		})
	}
}

// A malformed URL's error must not echo the password.
func TestLoadMQTTRedactsPassword(t *testing.T) {
	_, err := Load(envFunc(map[string]string{"GROWER_MQTT_URL": "mqtt://u:hunter2@host:port"}))
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Errorf("err = %v", err)
	}
}
