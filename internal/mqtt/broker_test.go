package mqtt

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/reilandeubank/grower/internal/config"
)

// TestBroker runs the publisher against a real broker, with a second client standing in
// for Home Assistant. Set GROWER_TEST_MQTT_URL (e.g. mqtt://localhost:1883) to run it.
func TestBroker(t *testing.T) {
	raw := os.Getenv("GROWER_TEST_MQTT_URL")
	if raw == "" {
		t.Skip("GROWER_TEST_MQTT_URL not set")
	}
	cfg, err := config.Load(func(k string) string {
		switch k {
		case "GROWER_MQTT_URL":
			return raw
		case "GROWER_MQTT_TOPIC":
			return "grower-test-" + strings.ToLower(t.Name())
		}
		return ""
	})
	must(t, err)

	f := newFixture(t)
	f.now = time.Now()
	fern := f.plant("Fern", "")
	today := time.Now().In(mustLoad(t, "America/Chicago")).Format(time.DateOnly)
	mist := f.task(fern, "mist", "", today, false)
	f.p = NewPublisher(f.d, NewTopics(cfg.MQTT.Topic, cfg.MQTT.DiscoveryPrefix), "https://grower.test", "test", time.Now, f.p.Log)
	topics := f.p.Topics

	// Home Assistant's side.
	var mu sync.Mutex
	got := map[string]string{}
	ha := paho.NewClient(paho.NewClientOptions().AddBroker(cfg.MQTT.Broker.String()).SetClientID("ha-" + topics.ID))
	if tok := ha.Connect(); !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("connect: %v", tok.Error())
	}
	defer ha.Disconnect(100)
	record := func(_ paho.Client, m paho.Message) {
		mu.Lock()
		got[m.Topic()] = string(m.Payload())
		mu.Unlock()
	}
	// Everything the publisher writes, so the cleanup below can clear it all.
	ha.SubscribeMultiple(map[string]byte{
		topics.Base + "/#": 1, topics.DeviceConfig(topics.ID): 1, topics.DeviceConfig(topics.PlantObjectID(fern)): 1,
	}, record).Wait()
	waitFor := func(what string, ok func(map[string]string) bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			done := ok(got)
			mu.Unlock()
			if done {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s; got %v", what, got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.p.Run(ctx, cfg.MQTT); close(done) }()

	waitFor("online, the household and Fern's device", func(m map[string]string) bool {
		return m[topics.Status()] == "online" && m[topics.DeviceConfig(topics.ID)] != "" && strings.Contains(m[topics.DeviceConfig(topics.PlantObjectID(fern))], `"name":"Fern"`) &&
			strings.Contains(m[topics.PlantState(fern)], `"needs_care":true`)
	})

	// Pressing the button marks the task done, and the new state comes back.
	ha.Publish(topics.TaskDone(mist), 1, false, pressPayload).Wait()
	waitFor("Fern done", func(m map[string]string) bool {
		return strings.Contains(m[topics.PlantState(fern)], `"needs_care":false`)
	})
	if n := f.countEvents(mist); n != 1 {
		t.Errorf("done events = %d", n)
	}

	// A change in the app shows up after a poke.
	f.exec(`UPDATE plants SET name = 'Boston fern' WHERE id = ?`, fern)
	f.p.Poke()
	waitFor("rename", func(m map[string]string) bool {
		return strings.Contains(m[topics.DeviceConfig(topics.PlantObjectID(fern))], `"name":"Boston fern"`)
	})

	cancel()
	<-done
	waitFor("offline", func(m map[string]string) bool { return m[topics.Status()] == "offline" })

	// Clean up the retained messages this test left.
	for topic := range got {
		ha.Publish(topic, 1, true, "").Wait()
	}
}

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	must(t, err)
	return loc
}
