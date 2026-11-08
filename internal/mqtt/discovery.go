// Package mqtt publishes grower's plants to Home Assistant over MQTT, with discovery:
// one device per plant with sensors and a button per care task, and a household device
// with the same summary the REST endpoint returns. It is off unless GROWER_MQTT_URL is set.
package mqtt

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

// supportURL is where Home Assistant's device page links for help with the integration.
const supportURL = "https://github.com/reilandeubank/grower"

// pressPayload is what a button sends. Any payload counts as a press.
const pressPayload = "PRESS"

// Topics names everything the publisher reads and writes.
type Topics struct {
	// Base prefixes grower's own topics (GROWER_MQTT_TOPIC); Discovery is Home
	// Assistant's discovery prefix.
	Base, Discovery string
	// ID is Base made safe for discovery object and unique IDs. It is the household
	// device's object ID and starts every plant's.
	ID string
}

var unsafeID = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// NewTopics derives the topics from grower's base topic and Home Assistant's prefix.
func NewTopics(base, discovery string) Topics {
	return Topics{Base: base, Discovery: discovery, ID: unsafeID.ReplaceAllString(base, "_")}
}

// Status carries "online" or "offline" (the last will), retained.
func (t Topics) Status() string { return t.Base + "/status" }

// Summary carries the household summary: the REST endpoint's JSON.
func (t Topics) Summary() string { return t.Base + "/summary" }

// PlantState carries one plant's state JSON.
func (t Topics) PlantState(plantID string) string { return t.Base + "/plant/" + plantID + "/state" }

// TaskDone is the command topic a task's button publishes to.
func (t Topics) TaskDone(taskID string) string { return t.Base + "/task/" + taskID + "/done" }

// TaskReading is the command topic a soil-check task's reading publishes to: the meter's
// value, 1 to 10.
func (t Topics) TaskReading(taskID string) string { return t.Base + "/task/" + taskID + "/reading" }

// HAStatus is where Home Assistant announces it has (re)started.
func (t Topics) HAStatus() string { return t.Discovery + "/status" }

// DeviceConfigs matches every device discovery topic, grower's and others'.
func (t Topics) DeviceConfigs() string { return t.Discovery + "/device/+/config" }

// DeviceConfig is a device's discovery topic.
func (t Topics) DeviceConfig(objectID string) string {
	return t.Discovery + "/device/" + objectID + "/config"
}

// PlantObjectID is a plant device's discovery object ID.
func (t Topics) PlantObjectID(plantID string) string { return t.ID + "_" + plantID }

// objectIDOf returns the object ID of a device discovery topic, if it is grower's own:
// the household device or a plant's.
func (t Topics) objectIDOf(topic string) (string, bool) {
	rest, ok := strings.CutPrefix(topic, t.Discovery+"/device/")
	if !ok {
		return "", false
	}
	id, ok := strings.CutSuffix(rest, "/config")
	if !ok || strings.Contains(id, "/") {
		return "", false
	}
	return id, id == t.ID || strings.HasPrefix(id, t.ID+"_")
}

// stateTopicOf is the state topic that goes with a device's object ID.
func (t Topics) stateTopicOf(objectID string) string {
	if objectID == t.ID {
		return t.Summary()
	}
	return t.PlantState(strings.TrimPrefix(objectID, t.ID+"_"))
}

// Commands a task's entities send.
const (
	cmdDone    = "done"
	cmdReading = "reading"
)

// commandOf returns the task and command (cmdDone or cmdReading) of a command topic.
func (t Topics) commandOf(topic string) (taskID, cmd string, ok bool) {
	rest, ok := strings.CutPrefix(topic, t.Base+"/task/")
	if !ok {
		return "", "", false
	}
	id, cmd, ok := strings.Cut(rest, "/")
	if !ok || id == "" || (cmd != cmdDone && cmd != cmdReading) {
		return "", "", false
	}
	return id, cmd, true
}

// component is one entity's discovery config; "platform" says which kind it is.
type component map[string]any

// device is one discovery message: the device and its entities by component key.
type device struct {
	info       map[string]any
	components map[string]component
}

// snapshot is everything that should be on the broker: device discovery by object ID,
// and state payloads by topic.
type snapshot struct {
	devices map[string]device
	states  map[string][]byte
}

// plantState is what a plant's state topic carries. Dates are YYYY-MM-DD; nulls mean
// never (Home Assistant shows them as unknown).
type plantState struct {
	NeedsCare   bool    `json:"needs_care"`
	NextDue     string  `json:"next_due"`
	DaysOverdue int     `json:"days_overdue"`
	LastWatered *string `json:"last_watered"`
	Moisture    *int    `json:"moisture"`
	// NeedsWater is true when a soil check's latest reading was at or below its water-at
	// level and the plant hasn't been watered since: the app's "time to water".
	NeedsWater bool                 `json:"needs_water"`
	Tasks      map[string]taskState `json:"tasks"`
}

type taskState struct {
	NextDue     string  `json:"next_due"`
	DaysOverdue int     `json:"days_overdue"`
	LastDone    *string `json:"last_done"`
	// Reading is a soil-check task's latest meter reading; null for other tasks, or
	// before the first reading.
	Reading *int `json:"reading"`
}

// build works out the snapshot for every scheduled task of an active plant (care.Tasks)
// and the household summary. rooms maps location IDs to names; baseURL and version link
// and label the devices.
func build(t Topics, items []care.Item, rooms map[string]string, summary apigen.HomeAssistantSummary, baseURL, version string) (snapshot, error) {
	snap := snapshot{devices: map[string]device{}, states: map[string][]byte{}}
	origin := map[string]any{"name": "grower", "sw_version": version, "support_url": supportURL}

	household := device{
		info: map[string]any{
			"identifiers": []string{t.ID}, "name": "grower", "manufacturer": "grower", "model": "Plant care",
			"sw_version": version, "configuration_url": baseURL,
		},
		components: map[string]component{},
	}
	for _, s := range []struct{ key, name, icon string }{
		{"needs_care", "Needs care", "mdi:sprout"},
		{"overdue", "Overdue", "mdi:calendar-alert"},
		{"upcoming", "Upcoming", "mdi:calendar-clock"},
	} {
		c := component{
			"platform": "sensor", "name": s.name, "unique_id": t.ID + "_" + s.key, "icon": s.icon,
			"state_topic": t.Summary(), "value_template": "{{ value_json." + s.key + " }}",
			"unit_of_measurement": "tasks", "state_class": "measurement",
		}
		if s.key == "needs_care" {
			// The whole summary, title, message and items included, as attributes.
			c["json_attributes_topic"] = t.Summary()
		}
		household.components[s.key] = c
	}
	snap.devices[t.ID] = household
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return snapshot{}, err
	}
	snap.states[t.Summary()] = summaryJSON

	// Group by plant, keeping care.Tasks' order (by plant name, then task order).
	var order []string
	byPlant := map[string][]care.Item{}
	names := map[string]map[string]bool{}
	for _, it := range items {
		if _, seen := byPlant[it.Plant.ID]; !seen {
			order = append(order, it.Plant.ID)
			key := strings.ToLower(it.PlantName)
			if names[key] == nil {
				names[key] = map[string]bool{}
			}
			names[key][it.Plant.ID] = true
		}
		byPlant[it.Plant.ID] = append(byPlant[it.Plant.ID], it)
	}
	for _, id := range order {
		tasks := byPlant[id]
		first := tasks[0]
		room := ""
		if first.Plant.LocationID.Valid {
			room = rooms[first.Plant.LocationID.String]
		}
		name := first.PlantName
		if len(names[strings.ToLower(name)]) > 1 && room != "" {
			name += " (" + room + ")"
		}
		obj := t.PlantObjectID(id)
		stateTopic := t.PlantState(id)
		info := map[string]any{
			"identifiers": []string{obj}, "name": name, "manufacturer": "grower", "model": "Plant",
			"via_device": t.ID, "configuration_url": strings.TrimSuffix(baseURL, "/") + "/plants/" + id,
		}
		if room != "" {
			info["suggested_area"] = room
		}
		comps := map[string]component{
			"needs_care": {
				"platform": "binary_sensor", "name": "Needs care", "device_class": "problem", "unique_id": obj + "_needs_care",
				"state_topic": stateTopic, "value_template": "{{ 'ON' if value_json.needs_care else 'OFF' }}",
			},
			"next_due": {
				"platform": "sensor", "name": "Next care", "device_class": "date", "unique_id": obj + "_next_due",
				"state_topic": stateTopic, "value_template": "{{ value_json.next_due }}",
			},
			"days_overdue": {
				"platform": "sensor", "name": "Days overdue", "device_class": "duration", "unit_of_measurement": "d",
				"unique_id": obj + "_days_overdue", "icon": "mdi:calendar-alert",
				"state_topic": stateTopic, "value_template": "{{ value_json.days_overdue }}",
			},
		}
		state := plantState{Tasks: map[string]taskState{}}
		var lastWatered, moistureOn string
		for i, it := range tasks {
			d := it.Dates
			ts := taskState{NextDue: d.Next.String(), DaysOverdue: it.DaysOverdue}
			if d.HasLast {
				last := d.Last.String()
				ts.LastDone = &last
				if it.Task.Kind == "water" && last > lastWatered {
					lastWatered = last
				}
			}
			if it.DaysOverdue >= 0 {
				state.NeedsCare = true
			}
			state.DaysOverdue = max(state.DaysOverdue, it.DaysOverdue)
			if i == 0 || ts.NextDue < state.NextDue {
				state.NextDue = ts.NextDue
			}
			if r := d.LastReading; it.Task.MoistureCheck == 1 && r.Reading > 0 {
				reading := r.Reading
				ts.Reading = &reading
				if r.On.String() >= moistureOn {
					moistureOn = r.On.String()
					state.Moisture = &reading
				}
				if needsWater(it) {
					state.NeedsWater = true
				}
			}
			state.Tasks[it.Task.ID] = ts

			due, done := taskNames(it.Task)
			comps[it.Task.ID+"_due"] = component{
				"platform": "sensor", "name": due, "device_class": "date", "unique_id": obj + "_" + it.Task.ID + "_due",
				"state_topic": stateTopic, "value_template": "{{ value_json.tasks['" + it.Task.ID + "'].next_due }}",
			}
			comps[it.Task.ID+"_done"] = component{
				"platform": "button", "name": done, "unique_id": obj + "_" + it.Task.ID + "_done", "icon": "mdi:check",
				"command_topic": t.TaskDone(it.Task.ID), "payload_press": pressPayload,
			}
			if it.Task.MoistureCheck == 1 {
				comps[it.Task.ID+"_reading"] = component{
					"platform": "number", "name": "Soil reading", "unique_id": obj + "_" + it.Task.ID + "_reading",
					"icon": "mdi:gauge", "min": 1, "max": 10, "step": 1, "mode": "box",
					"command_topic": t.TaskReading(it.Task.ID), "state_topic": stateTopic,
					// A task with no reading yet renders None, which Home Assistant reads as unknown.
					"value_template": "{{ value_json.tasks['" + it.Task.ID + "'].reading }}",
				}
			}
		}
		if lastWatered != "" {
			state.LastWatered = &lastWatered
		}
		if hasKind(tasks, "water") {
			comps["last_watered"] = component{
				"platform": "sensor", "name": "Last watered", "device_class": "date", "unique_id": obj + "_last_watered",
				"state_topic": stateTopic, "value_template": "{{ value_json.last_watered }}",
			}
		}
		if hasSoilCheck(tasks) {
			comps["moisture"] = component{
				"platform": "sensor", "name": "Soil moisture", "unique_id": obj + "_moisture", "icon": "mdi:water-percent",
				"state_class": "measurement", "state_topic": stateTopic, "value_template": "{{ value_json.moisture }}",
			}
			comps["needs_water"] = component{
				"platform": "binary_sensor", "name": "Needs water", "unique_id": obj + "_needs_water", "icon": "mdi:watering-can",
				"state_topic": stateTopic, "value_template": "{{ 'ON' if value_json.needs_water else 'OFF' }}",
			}
		}
		payload, err := json.Marshal(state)
		if err != nil {
			return snapshot{}, err
		}
		snap.devices[obj] = device{info: info, components: comps}
		snap.states[stateTopic] = payload
	}
	for obj, d := range snap.devices {
		d.info["origin"] = origin
		snap.devices[obj] = d
	}
	return snap, nil
}

// configPayload is a device's discovery message, with removal stubs (just the platform)
// for components Home Assistant still has but the device no longer does.
func configPayload(t Topics, d device, stale map[string]string) ([]byte, error) {
	comps := make(map[string]component, len(d.components)+len(stale))
	for k, c := range d.components {
		comps[k] = c
	}
	for k, platform := range stale {
		if _, ok := comps[k]; !ok {
			comps[k] = component{"platform": platform}
		}
	}
	info := make(map[string]any, len(d.info))
	for k, v := range d.info {
		if k != "origin" {
			info[k] = v
		}
	}
	return json.Marshal(map[string]any{
		"device":             info,
		"origin":             d.info["origin"],
		"availability_topic": t.Status(),
		"components":         comps,
	})
}

// parseConfig reads a discovery message received from the broker: its origin, and the
// platform of each live (not removal-stub) component.
func parseConfig(payload []byte) (origin string, live map[string]string, err error) {
	var msg struct {
		Origin struct {
			Name string `json:"name"`
		} `json:"origin"`
		Components map[string]map[string]any `json:"components"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return "", nil, err
	}
	live = map[string]string{}
	for k, c := range msg.Components {
		platform, _ := c["platform"].(string)
		if platform != "" && len(c) > 1 {
			live[k] = platform
		}
	}
	return msg.Origin.Name, live, nil
}

// taskNames are a task's entity names within its plant's device: its due-date sensor and
// its button. A soil-check task's button logs a watering.
func taskNames(t dbgen.CareTask) (due, done string) {
	if t.MoistureCheck == 1 {
		return "Next soil check", "Watered"
	}
	name, ok := map[string]string{
		"water": "Water", "fertilize": "Fertilize", "mist": "Mist", "repot": "Repot", "rotate": "Rotate", "check_in": "Check-in",
	}[t.Kind]
	if !ok {
		name = strings.TrimSpace(t.Label)
		if name == "" {
			name = "Task"
		}
	}
	return name + " due", name + " done"
}

func hasKind(items []care.Item, kind string) bool {
	for _, it := range items {
		if it.Task.Kind == kind {
			return true
		}
	}
	return false
}

// needsWater reports whether a soil-check task's latest reading said to water (at or
// below its water-at level, as the app's verdict) with no watering logged since. A
// watering logged the same day as the reading counts as after it.
func needsWater(it care.Item) bool {
	r := it.Dates.LastReading
	if it.Task.MoistureCheck != 1 || r.Kind != schedule.Checked || r.Reading <= 0 || int64(r.Reading) > it.Task.WaterAt {
		return false
	}
	return !it.Dates.HasLast || it.Dates.Last.Before(r.On)
}

func hasSoilCheck(items []care.Item) bool {
	for _, it := range items {
		if it.Task.MoistureCheck == 1 {
			return true
		}
	}
	return false
}
