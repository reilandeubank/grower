package mqtt

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/hass"
	"github.com/reilandeubank/grower/internal/schedule"
)

// Client publishes a retained message at QoS 1. An empty payload clears the topic.
type Client interface {
	Publish(topic string, payload []byte) error
}

// Publisher keeps the broker in step with the database. Its methods other than Poke
// must be called from one goroutine (Run's).
type Publisher struct {
	DB      *db.DB
	Topics  Topics
	BaseURL string
	Version string
	Now     func() time.Time
	Log     *slog.Logger

	pokes chan struct{}
	// sent is the last payload sent on each topic, so unchanged ones aren't resent.
	sent map[string][]byte
	// known is, by device object ID, the components Home Assistant has (key to
	// platform): what was last published, plus anything learned from retained
	// discovery messages, as from before a restart.
	known map[string]map[string]string
}

// NewPublisher returns a publisher ready to Run.
func NewPublisher(d *db.DB, t Topics, baseURL, version string, now func() time.Time, log *slog.Logger) *Publisher {
	return &Publisher{
		DB: d, Topics: t, BaseURL: baseURL, Version: version, Now: now, Log: log,
		pokes: make(chan struct{}, 1), sent: map[string][]byte{}, known: map[string]map[string]string{},
	}
}

// Poke asks for a sync soon, as after the app changes something. It never blocks, and
// pokes before a sync starts coalesce. Safe on a nil Publisher.
func (p *Publisher) Poke() {
	if p == nil {
		return
	}
	select {
	case p.pokes <- struct{}{}:
	default:
	}
}

// forget drops what was sent, so the next sync republishes everything: after a reconnect
// (the broker may have lost retained messages) or a Home Assistant restart.
func (p *Publisher) forget() { clear(p.sent) }

// calendar is the household's: the first account's time zone and hemisphere. ok is
// false before any account exists.
func (p *Publisher) calendar(ctx context.Context) (cal care.Calendar, ok bool, err error) {
	row, err := p.DB.Reader().GetHouseholdSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return care.Calendar{}, false, nil
	}
	if err != nil {
		return care.Calendar{}, false, fmt.Errorf("household settings: %w", err)
	}
	loc, err := time.LoadLocation(row.Setting.Timezone)
	if err != nil {
		return care.Calendar{}, false, fmt.Errorf("user %s time zone: %w", row.Setting.UserID, err)
	}
	return care.Calendar{Today: schedule.Today(p.Now(), loc), Hemisphere: schedule.Hemisphere(row.Setting.Hemisphere)}, true, nil
}

// Sync publishes whatever changed since the last sync: discovery for new or changed
// devices (removal stubs for entities that went away), empty retained messages for
// devices that went away, and changed state. A failed publish stops the sync; the
// next one retries it.
func (p *Publisher) Sync(ctx context.Context, c Client) error {
	cal, ok, err := p.calendar(ctx)
	if err != nil || !ok {
		return err
	}
	q := p.DB.Reader()
	items, err := care.Tasks(ctx, q, cal)
	if err != nil {
		return err
	}
	rooms, err := hass.Rooms(ctx, q)
	if err != nil {
		return err
	}
	snap, err := build(p.Topics, items, rooms, hass.Summary(care.Split(cal.Today, items), rooms), p.BaseURL, p.Version)
	if err != nil {
		return err
	}

	for obj := range p.known {
		if _, ok := snap.devices[obj]; ok {
			continue
		}
		for _, topic := range []string{p.Topics.DeviceConfig(obj), p.Topics.stateTopicOf(obj)} {
			if err := c.Publish(topic, nil); err != nil {
				return fmt.Errorf("remove %s: %w", topic, err)
			}
			delete(p.sent, topic)
		}
		delete(p.known, obj)
		p.Log.InfoContext(ctx, "mqtt: removed device", "object_id", obj)
	}
	for _, obj := range sortedKeys(snap.devices) {
		d := snap.devices[obj]
		payload, err := configPayload(p.Topics, d, p.known[obj])
		if err != nil {
			return err
		}
		if err := p.send(c, p.Topics.DeviceConfig(obj), payload); err != nil {
			return err
		}
		current := make(map[string]string, len(d.components))
		for k, comp := range d.components {
			current[k], _ = comp["platform"].(string)
		}
		p.known[obj] = current
	}
	for _, topic := range sortedKeys(snap.states) {
		if err := p.send(c, topic, snap.states[topic]); err != nil {
			return err
		}
	}
	return nil
}

// send publishes payload on topic unless it was the last thing sent there.
func (p *Publisher) send(c Client, topic string, payload []byte) error {
	if prev, ok := p.sent[topic]; ok && bytes.Equal(prev, payload) {
		return nil
	}
	if err := c.Publish(topic, payload); err != nil {
		return fmt.Errorf("publish %s: %w", topic, err)
	}
	p.sent[topic] = payload
	return nil
}

// Discovered takes a device discovery message from the broker (DeviceConfigs) and
// records the components of grower's own devices, so ones from before a restart are
// removed when they no longer exist. It reports whether a sync is needed.
func (p *Publisher) Discovered(topic string, payload []byte) bool {
	obj, ours := p.Topics.objectIDOf(topic)
	if !ours || len(payload) == 0 {
		return false
	}
	origin, live, err := parseConfig(payload)
	if err != nil || origin != "grower" {
		return false
	}
	known := p.known[obj]
	if known == nil {
		known = map[string]string{}
	}
	changed := false
	for k, platform := range live {
		if _, ok := known[k]; !ok {
			known[k] = platform
			changed = true
		}
	}
	p.known[obj] = known
	if changed {
		// Republish the device with stubs for what it no longer has.
		delete(p.sent, topic)
	}
	return changed
}

// Pressed handles a message on a button's command topic: it marks the task done today,
// unless it already was (a double tap, or an automation firing twice). Retained
// messages are ignored, so a stray one can't mark a task done on every reconnect. It
// reports whether a sync is needed.
func (p *Publisher) Pressed(ctx context.Context, topic string, retained bool) (bool, error) {
	taskID, ok := p.Topics.taskIDOf(topic)
	if !ok {
		return false, nil
	}
	if retained {
		p.Log.WarnContext(ctx, "mqtt: ignored a retained button press; clear it from the broker", "topic", topic)
		return false, nil
	}
	cal, ok, err := p.calendar(ctx)
	if err != nil || !ok {
		return false, err
	}
	items, err := care.Tasks(ctx, p.DB.Reader(), cal)
	if err != nil {
		return false, err
	}
	var item *care.Item
	for i := range items {
		if items[i].Task.ID == taskID {
			item = &items[i]
			break
		}
	}
	switch {
	case item == nil:
		p.Log.InfoContext(ctx, "mqtt: button pressed for a task that no longer exists or is archived", "task_id", taskID)
		return false, nil
	case item.Dates.HasLast && item.Dates.Last == cal.Today:
		p.Log.InfoContext(ctx, "mqtt: task already done today; ignored the press", "task_id", taskID)
		return false, nil
	}
	now := db.FormatTime(p.Now())
	_, err = p.DB.Writer().CreateEvent(ctx, dbgen.CreateEventParams{
		ID: db.NewID(), TaskID: taskID, Kind: "done", LocalDate: cal.Today.String(), OccurredAt: now, CreatedAt: now,
	})
	if err != nil {
		return false, fmt.Errorf("mark task %s done: %w", taskID, err)
	}
	p.Log.InfoContext(ctx, "mqtt: marked task done from Home Assistant", "task_id", taskID, "plant", item.PlantName)
	return true, nil
}

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }
