package mqtt

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/db/dbtest"
)

const stamp = "2026-01-01T00:00:00Z"

// fakeBroker keeps retained messages as a broker would, and logs every publish.
type fakeBroker struct {
	retained map[string][]byte
	log      []string
}

func (b *fakeBroker) Publish(topic string, payload []byte) error {
	if len(payload) == 0 {
		delete(b.retained, topic)
	} else {
		b.retained[topic] = slices.Clone(payload)
	}
	b.log = append(b.log, topic)
	return nil
}

// published returns and resets the topics published since the last call.
func (b *fakeBroker) published() []string {
	out := b.log
	b.log = nil
	slices.Sort(out)
	return out
}

type fixture struct {
	t      *testing.T
	d      *db.DB
	p      *Publisher
	broker *fakeBroker
	now    time.Time
}

// newFixture has one account in America/Chicago, and the clock at noon on 2026-05-01.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, d: dbtest.Open(t), broker: &fakeBroker{retained: map[string][]byte{}}}
	f.now = time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC)
	f.p = f.publisher()
	ctx, q := context.Background(), f.d.Writer()
	userID := db.NewID()
	must(t, q.CreateUser(ctx, dbgen.CreateUserParams{ID: userID, Username: "fern", PasswordHash: "x", CreatedAt: stamp, UpdatedAt: stamp}))
	must(t, q.CreateSettings(ctx, dbgen.CreateSettingsParams{UserID: userID, Timezone: "America/Chicago", CreatedAt: stamp, UpdatedAt: stamp}))
	return f
}

// publisher is a fresh publisher on the fixture's database, as after a restart.
func (f *fixture) publisher() *Publisher {
	return NewPublisher(f.d, NewTopics("grower", "homeassistant"), "https://grower.test", "1.2.3", func() time.Time { return f.now }, slog.New(slog.DiscardHandler))
}

func (f *fixture) sync() []string {
	f.t.Helper()
	must(f.t, f.p.Sync(context.Background(), f.broker))
	return f.broker.published()
}

func (f *fixture) room(name string) string {
	f.t.Helper()
	id := db.NewID()
	must(f.t, f.d.Writer().CreateLocation(context.Background(), dbgen.CreateLocationParams{ID: id, Name: name, CreatedAt: stamp, UpdatedAt: stamp}))
	return id
}

func (f *fixture) plant(name, room string) string {
	f.t.Helper()
	p, err := f.d.Writer().CreatePlant(context.Background(), dbgen.CreatePlantParams{
		ID: db.NewID(), Name: name, LocationID: sql.NullString{String: room, Valid: room != ""}, CreatedAt: stamp, UpdatedAt: stamp,
	})
	must(f.t, err)
	return p.ID
}

// task adds a task every 7 days from start; soil makes it a soil-check watering.
func (f *fixture) task(plantID, kind, label, start string, soil bool) string {
	f.t.Helper()
	var moisture int64
	if soil {
		moisture = 1
	}
	tk, err := f.d.Writer().CreateTask(context.Background(), dbgen.CreateTaskParams{
		ID: db.NewID(), PlantID: plantID, Kind: kind, Label: label, IntervalDays: 7, Anchor: "fixed", StartOn: start,
		MoistureCheck: moisture, WaterAt: 3, CreatedAt: stamp, UpdatedAt: stamp,
	})
	must(f.t, err)
	return tk.ID
}

func (f *fixture) event(taskID, kind, on string, reading int64) {
	f.t.Helper()
	_, err := f.d.Writer().CreateEvent(context.Background(), dbgen.CreateEventParams{
		ID: db.NewID(), TaskID: taskID, Kind: kind, LocalDate: on, Reading: sql.NullInt64{Int64: reading, Valid: reading > 0},
		OccurredAt: stamp, CreatedAt: stamp,
	})
	must(f.t, err)
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.d.Write.Exec(query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) countEvents(taskID string) int {
	f.t.Helper()
	var n int
	must(f.t, f.d.Read.QueryRow(`SELECT count(*) FROM care_events WHERE task_id = ? AND kind = 'done'`, taskID).Scan(&n))
	return n
}

// discovery is a device's retained discovery message, decoded.
type discovery struct {
	Device struct {
		Identifiers   []string `json:"identifiers"`
		Name          string   `json:"name"`
		SuggestedArea string   `json:"suggested_area"`
		ViaDevice     string   `json:"via_device"`
		ConfigURL     string   `json:"configuration_url"`
	} `json:"device"`
	Origin struct {
		Name    string `json:"name"`
		Version string `json:"sw_version"`
	} `json:"origin"`
	Availability string                    `json:"availability_topic"`
	Components   map[string]map[string]any `json:"components"`
}

func (f *fixture) discovery(objectID string) discovery {
	f.t.Helper()
	raw, ok := f.broker.retained["homeassistant/device/"+objectID+"/config"]
	if !ok {
		f.t.Fatalf("no discovery for %s", objectID)
	}
	var d discovery
	must(f.t, json.Unmarshal(raw, &d))
	return d
}

func (f *fixture) state(plantID string) plantState {
	f.t.Helper()
	var s plantState
	must(f.t, json.Unmarshal(f.broker.retained["grower/plant/"+plantID+"/state"], &s))
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func keys(m map[string]map[string]any) string {
	return strings.Join(sortedKeys(m), ",")
}

func TestSyncWithoutAccountPublishesNothing(t *testing.T) {
	f := &fixture{t: t, d: dbtest.Open(t), broker: &fakeBroker{retained: map[string][]byte{}}}
	f.p = f.publisher()
	if got := f.sync(); len(got) != 0 {
		t.Errorf("published %v", got)
	}
}

func TestSyncPublishesDevicesAndState(t *testing.T) {
	f := newFixture(t)
	office, living := f.room("Office"), f.room("Living room")
	officePothos, livingPothos, fern := f.plant("Pothos", office), f.plant("Pothos", living), f.plant("Fern", "")
	water := f.task(officePothos, "water", "", "2026-04-24", false)
	f.event(water, "done", "2026-04-20", 0)
	soil := f.task(livingPothos, "water", "", "2026-04-30", true)
	f.event(soil, "checked", "2026-04-29", 6)
	mist := f.task(fern, "mist", "", "2026-05-01", false)
	prune := f.task(fern, "custom", "Prune", "2026-05-20", false)

	f.sync()

	// The household device carries the summary.
	h := f.discovery("grower")
	if h.Device.Name != "grower" || h.Origin.Name != "grower" || h.Origin.Version != "1.2.3" || h.Availability != "grower/status" {
		t.Errorf("household = %+v", h)
	}
	if got := keys(h.Components); got != "needs_care,overdue,upcoming" {
		t.Errorf("household components = %s", got)
	}
	if c := h.Components["needs_care"]; c["state_topic"] != "grower/summary" || c["json_attributes_topic"] != "grower/summary" {
		t.Errorf("needs_care = %v", c)
	}
	var summary struct {
		NeedsCare int    `json:"needs_care"`
		Title     string `json:"title"`
	}
	must(t, json.Unmarshal(f.broker.retained["grower/summary"], &summary))
	if summary.NeedsCare != 3 || summary.Title != "3 plants need care" {
		t.Errorf("summary = %+v", summary)
	}

	// Plants that share a name are told apart by room, and land in that area.
	op := f.discovery("grower_" + officePothos)
	if op.Device.Name != "Pothos (Office)" || op.Device.SuggestedArea != "Office" || op.Device.ViaDevice != "grower" ||
		op.Device.ConfigURL != "https://grower.test/plants/"+officePothos || op.Device.Identifiers[0] != "grower_"+officePothos {
		t.Errorf("office pothos device = %+v", op.Device)
	}
	if got, want := keys(op.Components), "days_overdue,last_watered,needs_care,next_due,"+water+"_done,"+water+"_due"; got != sortedJoin(want) {
		t.Errorf("office pothos components = %s, want %s", got, sortedJoin(want))
	}
	button := op.Components[water+"_done"]
	if button["platform"] != "button" || button["name"] != "Water done" || button["command_topic"] != "grower/task/"+water+"/done" {
		t.Errorf("button = %v", button)
	}
	if s := f.state(officePothos); !s.NeedsCare || s.DaysOverdue != 7 || s.NextDue != "2026-04-24" || s.LastWatered == nil || *s.LastWatered != "2026-04-20" {
		t.Errorf("office pothos state = %+v", s)
	}

	// A soil-check watering gets a moisture sensor and a "Watered" button.
	lp := f.discovery("grower_" + livingPothos)
	if lp.Device.Name != "Pothos (Living room)" || lp.Components["moisture"] == nil || lp.Components[soil+"_done"]["name"] != "Watered" ||
		lp.Components[soil+"_due"]["name"] != "Next soil check" {
		t.Errorf("living pothos = %+v", lp)
	}
	if s := f.state(livingPothos); s.Moisture == nil || *s.Moisture != 6 || s.LastWatered != nil {
		t.Errorf("living pothos state = %+v", s)
	}

	// Fern: no watering, so no last-watered sensor; a custom task is named by its label.
	fd := f.discovery("grower_" + fern)
	if fd.Device.Name != "Fern" || fd.Device.SuggestedArea != "" || fd.Components["last_watered"] != nil || fd.Components["moisture"] != nil {
		t.Errorf("fern = %+v", fd)
	}
	if fd.Components[prune+"_due"]["name"] != "Prune due" || fd.Components[prune+"_done"]["name"] != "Prune done" {
		t.Errorf("prune components = %v / %v", fd.Components[prune+"_due"], fd.Components[prune+"_done"])
	}
	s := f.state(fern)
	if !s.NeedsCare || s.DaysOverdue != 0 || s.NextDue != "2026-05-01" || s.Tasks[prune].NextDue != "2026-05-20" || s.Tasks[mist].LastDone != nil {
		t.Errorf("fern state = %+v", s)
	}

	// Nothing changed, nothing sent.
	if got := f.sync(); len(got) != 0 {
		t.Errorf("second sync published %v", got)
	}
}

func TestButtonMarksTaskDone(t *testing.T) {
	f := newFixture(t)
	fern := f.plant("Fern", "")
	mist := f.task(fern, "mist", "", "2026-05-01", false)
	f.sync()
	ctx := context.Background()
	press := func(topic string, retained bool) bool {
		t.Helper()
		changed, err := f.p.Pressed(ctx, topic, retained)
		must(t, err)
		return changed
	}

	// A retained press is ignored, so it can't fire again on every reconnect.
	if press("grower/task/"+mist+"/done", true) || f.countEvents(mist) != 0 {
		t.Fatal("a retained press marked the task done")
	}
	if !press("grower/task/"+mist+"/done", false) || f.countEvents(mist) != 1 {
		t.Fatal("press didn't mark the task done")
	}
	// Only the state that changed is republished, not discovery.
	if got := strings.Join(f.sync(), " "); got != "grower/plant/"+fern+"/state grower/summary" {
		t.Errorf("after press, published %s", got)
	}
	if s := f.state(fern); s.NeedsCare || s.Tasks[mist].LastDone == nil || *s.Tasks[mist].LastDone != "2026-05-01" || s.NextDue != "2026-05-08" {
		t.Errorf("state after press = %+v", s)
	}
	// A second press the same day (a double tap) does nothing.
	if press("grower/task/"+mist+"/done", false) || f.countEvents(mist) != 1 {
		t.Error("second press on the same day logged again")
	}
	// Unknown, archived, or malformed topics are ignored.
	if press("grower/task/nope/done", false) || press("grower/task//done", false) || press("grower/task/"+mist+"/undo", false) {
		t.Error("ignored press reported a change")
	}
	f.exec(`UPDATE plants SET archived_at = ? WHERE id = ?`, stamp, fern)
	f.now = f.now.Add(7 * 24 * time.Hour)
	if press("grower/task/"+mist+"/done", false) || f.countEvents(mist) != 1 {
		t.Error("pressed an archived plant's task")
	}
}

func TestRemovals(t *testing.T) {
	f := newFixture(t)
	fern, ivy := f.plant("Fern", ""), f.plant("Ivy", "")
	mist := f.task(fern, "mist", "", "2026-05-01", false)
	water := f.task(fern, "water", "", "2026-05-03", false)
	f.task(ivy, "water", "", "2026-05-03", false)
	f.sync()

	// A deleted task's entities are removed with stubs that carry only the platform.
	f.exec(`DELETE FROM care_tasks WHERE id = ?`, water)
	f.sync()
	fd := f.discovery("grower_" + fern)
	if stub := fd.Components[water+"_done"]; len(stub) != 1 || stub["platform"] != "button" {
		t.Errorf("removed button = %v", stub)
	}
	if stub := fd.Components["last_watered"]; len(stub) != 1 || stub["platform"] != "sensor" {
		t.Errorf("removed last_watered = %v", stub)
	}
	if fd.Components[mist+"_done"]["command_topic"] == nil {
		t.Error("kept task lost its button")
	}
	// The stubs go once: the next change publishes without them.
	f.event(mist, "done", "2026-05-01", 0)
	f.sync()
	if fd := f.discovery("grower_" + fern); fd.Components[water+"_done"] != nil {
		t.Errorf("stub sent again: %v", keys(fd.Components))
	}

	// An archived plant's device and state are cleared.
	f.exec(`UPDATE plants SET archived_at = ? WHERE id = ?`, stamp, ivy)
	got := f.sync()
	if !slices.Contains(got, "homeassistant/device/grower_"+ivy+"/config") || !slices.Contains(got, "grower/plant/"+ivy+"/state") {
		t.Errorf("archive published %v", got)
	}
	for _, topic := range []string{"homeassistant/device/grower_" + ivy + "/config", "grower/plant/" + ivy + "/state"} {
		if _, ok := f.broker.retained[topic]; ok {
			t.Errorf("%s still retained", topic)
		}
	}
	if got := f.sync(); len(got) != 0 {
		t.Errorf("removal repeated: %v", got)
	}
}

// After a restart, retained discovery tells the publisher what Home Assistant still has,
// so plants deleted while grower was down are cleared and other devices are left alone.
func TestRestartClearsWhatWentAway(t *testing.T) {
	f := newFixture(t)
	fern, ivy := f.plant("Fern", ""), f.plant("Ivy", "")
	mist := f.task(fern, "mist", "", "2026-05-01", false)
	water := f.task(fern, "water", "", "2026-05-01", false)
	f.task(ivy, "water", "", "2026-05-01", false)
	f.sync()
	before := map[string][]byte{}
	for topic, payload := range f.broker.retained {
		before[topic] = payload
	}

	// grower is down while Ivy and Fern's watering are deleted.
	f.exec(`DELETE FROM plants WHERE id = ?`, ivy)
	f.exec(`DELETE FROM care_tasks WHERE id = ?`, water)
	f.p = f.publisher()
	f.sync()

	// The broker replays retained discovery on subscribe: grower's old messages, and
	// another integration's device, which must be left alone.
	other := []byte(`{"origin":{"name":"zigbee2mqtt"},"components":{"x":{"platform":"sensor","name":"x"}}}`)
	f.broker.retained["homeassistant/device/grower_ZIGBEE/config"] = other
	needSync := false
	for _, topic := range sortedKeys(before) {
		if strings.HasPrefix(topic, "homeassistant/device/") {
			needSync = f.p.Discovered(topic, before[topic]) || needSync
		}
	}
	if f.p.Discovered("homeassistant/device/grower_ZIGBEE/config", other) || f.p.Discovered("homeassistant/device/lamp/config", other) {
		t.Error("another integration's device counted as grower's")
	}
	if !needSync {
		t.Fatal("stale retained discovery didn't ask for a sync")
	}
	f.sync()
	if _, ok := f.broker.retained["homeassistant/device/grower_"+ivy+"/config"]; ok {
		t.Error("deleted plant's device still retained")
	}
	if _, ok := f.broker.retained["grower/plant/"+ivy+"/state"]; ok {
		t.Error("deleted plant's state still retained")
	}
	if _, ok := f.broker.retained["homeassistant/device/grower_ZIGBEE/config"]; !ok {
		t.Error("cleared another integration's device")
	}
	fd := f.discovery("grower_" + fern)
	if stub := fd.Components[water+"_done"]; len(stub) != 1 {
		t.Errorf("deleted task's button = %v, want a removal stub", stub)
	}
	if fd.Components[mist+"_done"]["command_topic"] == nil {
		t.Error("kept task lost its button")
	}
	// Echoes of what was just published change nothing.
	for topic, payload := range f.broker.retained {
		if f.p.Discovered(topic, payload) {
			t.Errorf("echo of %s asked for a sync", topic)
		}
	}
}

func TestReconnectRepublishes(t *testing.T) {
	f := newFixture(t)
	f.task(f.plant("Fern", ""), "mist", "", "2026-05-01", false)
	first := f.sync()
	if len(f.sync()) != 0 {
		t.Fatal("unchanged sync published")
	}
	f.p.forget()
	if got := f.sync(); !slices.Equal(got, first) {
		t.Errorf("after forget published %v, want %v", got, first)
	}
}

func TestTopics(t *testing.T) {
	tp := NewTopics("home/plants", "ha")
	if tp.ID != "home_plants" || tp.PlantObjectID("01ABC") != "home_plants_01ABC" || tp.Status() != "home/plants/status" {
		t.Errorf("topics = %+v", tp)
	}
	for _, tt := range []struct {
		topic string
		id    string
		ours  bool
	}{
		{"ha/device/home_plants/config", "home_plants", true},
		{"ha/device/home_plants_01ABC/config", "home_plants_01ABC", true},
		{"ha/device/home_plantsX/config", "home_plantsX", false},
		{"ha/device/lamp/config", "lamp", false},
		{"other/device/home_plants/config", "", false},
	} {
		if id, ours := tp.objectIDOf(tt.topic); id != tt.id || ours != tt.ours {
			t.Errorf("objectIDOf(%s) = %q %v", tt.topic, id, ours)
		}
	}
	if tp.stateTopicOf("home_plants_01ABC") != "home/plants/plant/01ABC/state" || tp.stateTopicOf("home_plants") != "home/plants/summary" {
		t.Error("stateTopicOf")
	}
}

func TestPokeCoalescesAndIsNilSafe(t *testing.T) {
	var nilPub *Publisher
	nilPub.Poke()
	p := NewPublisher(nil, NewTopics("grower", "homeassistant"), "", "", time.Now, slog.New(slog.DiscardHandler))
	p.Poke()
	p.Poke()
	if len(p.pokes) != 1 {
		t.Errorf("pending pokes = %d, want 1", len(p.pokes))
	}
}

func sortedJoin(csv string) string {
	parts := strings.Split(csv, ",")
	slices.Sort(parts)
	return strings.Join(parts, ",")
}
