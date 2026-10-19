package server

import (
	"net/http"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
)

// TestMoistureCheckLoop: a soil-check task is due when to check; readings move the next
// check, and a watering after a dry reading teaches it how long the soil takes to dry.
func TestMoistureCheckLoop(t *testing.T) {
	e := newEnv(t, envOptions{}) // 2026-05-01 in Chicago
	c := e.loginCookie()
	p := e.newPlant(c, "Ficus")
	task := e.newTask(c, p.Id, `{"kind":"water","interval_days":7,"moisture_check":true}`)
	if !task.MoistureCheck || task.WaterAt != 3 || task.Anchor != apigen.FromCompletion || task.NextDue.String() != "2026-05-01" ||
		task.DryingDays != nil || task.LastReading != nil || task.CurrentIntervalDays != 7 {
		t.Fatalf("new task = %+v", task)
	}
	check := func(step string, got apigen.CareTask, next string, drying, reading int) {
		t.Helper()
		var gotDrying, gotReading int
		if got.DryingDays != nil {
			gotDrying = *got.DryingDays
		}
		if got.LastReading != nil {
			gotReading = got.LastReading.Value
		}
		if got.NextDue.String() != next || gotDrying != drying || gotReading != reading {
			t.Errorf("%s: next %s, drying %d, reading %d; want %s, %d, %d", step, got.NextDue, gotDrying, gotReading, next, drying, reading)
		}
	}

	// Watered without a reading: nothing learned yet, so check after interval_days.
	check("watered", e.logEvent(c, task.Id, `{"kind":"done"}`).Task, "2026-05-08", 0, 0)

	// Three days on it reads 6: 4 points in 3 days, 3 more to go → check in 3 days.
	e.clock.t = e.clock.t.AddDate(0, 0, 3)
	moist := e.logEvent(c, task.Id, `{"kind":"checked","reading":6}`)
	if moist.Event.Kind != apigen.Checked || moist.Event.Reading == nil || *moist.Event.Reading != 6 {
		t.Errorf("check event = %+v", moist.Event)
	}
	check("moist", moist.Task, "2026-05-07", 0, 6)
	if r := moist.Task.LastReading; r.On.String() != "2026-05-04" {
		t.Errorf("last reading on %s", r.On)
	}
	if s := summary(e.today(c).Upcoming); s != "Ficus/water:-3" {
		t.Errorf("today after the check: upcoming %q", s)
	}

	// Undoing the check restores the date from the watering.
	undone := expectJSON[apigen.CareTask](t, e.call(c, http.MethodDelete, "/api/v1/events/"+moist.Event.Id, ""), http.StatusOK)
	check("undone", undone, "2026-05-08", 0, 0)
	check("checked again", e.logEvent(c, task.Id, `{"kind":"checked","reading":6}`).Task, "2026-05-07", 0, 6)

	// On the check date it's due, reads 2, and is watered in one request.
	e.clock.t = e.clock.t.AddDate(0, 0, 3)
	if s := summary(e.today(c).Due); s != "Ficus/water:0" {
		t.Errorf("today on the check date: due %q", s)
	}
	watered := e.logEvent(c, task.Id, `{"kind":"done","reading":2}`)
	check("watered when dry", watered.Task, "2026-05-13", 6, 2)
	if watered.Task.CurrentIntervalDays != 6 || watered.Task.LastDone.String() != "2026-05-07" ||
		watered.Event.Reading == nil || *watered.Event.Reading != 2 {
		t.Errorf("after watering = %+v", watered)
	}
	if s := summary(e.today(c).Upcoming); s != "Ficus/water:-6" {
		t.Errorf("today after watering: upcoming %q", s)
	}

	history := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events", ""), http.StatusOK)
	var readings []int
	for _, h := range history.Items {
		if h.Event.Reading != nil {
			readings = append(readings, *h.Event.Reading)
		}
	}
	if len(history.Items) != 3 || len(readings) != 2 || readings[0] != 2 || readings[1] != 6 {
		t.Errorf("history = %+v", history.Items)
	}

	// Dry again after 5 days: learned from both cycles (lower median of 6 and 5); not
	// watered yet, so check again tomorrow.
	e.clock.t = e.clock.t.AddDate(0, 0, 5)
	check("dry, not watered", e.logEvent(c, task.Id, `{"kind":"checked","reading":3}`).Task, "2026-05-13", 5, 3)
}

func TestMoistureValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Ficus")
	soil := e.newTask(c, p.Id, `{"kind":"water","interval_days":7,"moisture_check":true,"water_at":2}`)
	plain := e.newTask(c, p.Id, `{"kind":"water","interval_days":7}`)
	if soil.WaterAt != 2 || plain.MoistureCheck || plain.WaterAt != 3 || plain.DryingDays != nil {
		t.Errorf("soil = %+v, plain = %+v", soil, plain)
	}
	tasks, soilEvents, plainEvents := "/api/v1/plants/"+p.Id+"/tasks", "/api/v1/tasks/"+soil.Id+"/events", "/api/v1/tasks/"+plain.Id+"/events"
	tests := []struct {
		name, method, path, body string
		detail                   string
	}{
		{"soil check on mist", http.MethodPost, tasks, `{"kind":"mist","interval_days":3,"moisture_check":true}`, "Only watering tasks can use soil checks."},
		{"water_at 0", http.MethodPost, tasks, `{"kind":"water","interval_days":3,"moisture_check":true,"water_at":0}`, "Water-at must be between 1 and 9."},
		{"water_at 10", http.MethodPut, "/api/v1/tasks/" + soil.Id, `{"kind":"water","interval_days":3,"moisture_check":true,"water_at":10}`, "Water-at must be between 1 and 9."},
		{"soil check on a fixed anchor", http.MethodPost, tasks, `{"kind":"water","interval_days":3,"moisture_check":true,"anchor":"fixed"}`, "Soil checks always count from the last watering."},

		{"check without a reading", http.MethodPost, soilEvents, `{"kind":"checked"}`, "Add the meter reading."},
		{"reading on a snooze", http.MethodPost, soilEvents, `{"kind":"snoozed","snooze_until":"2026-05-03","reading":4}`, "Only a check or a watering takes a reading."},
		{"reading on a skip", http.MethodPost, soilEvents, `{"kind":"skipped","reading":4}`, "Only a check or a watering takes a reading."},
		{"reading 0", http.MethodPost, soilEvents, `{"kind":"checked","reading":0}`, "Reading must be 1 to 10."},
		{"reading 11", http.MethodPost, soilEvents, `{"kind":"done","reading":11}`, "Reading must be 1 to 10."},
		{"check on a plain task", http.MethodPost, plainEvents, `{"kind":"checked","reading":4}`, "Only soil-check tasks take readings."},
		{"reading on a plain done", http.MethodPost, plainEvents, `{"kind":"done","reading":4}`, "Only soil-check tasks take readings."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectProblem(t, e.call(c, tt.method, tt.path, tt.body), http.StatusBadRequest); got != tt.detail {
				t.Errorf("detail = %q, want %q", got, tt.detail)
			}
		})
	}
	history := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events", ""), http.StatusOK)
	if len(history.Items) != 0 {
		t.Errorf("rejected events were stored: %+v", history.Items)
	}

	// Turning soil checks on resets an omitted anchor to from_completion; off resets water_at.
	fixed := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+plain.Id,
		`{"kind":"water","interval_days":7,"anchor":"fixed"}`), http.StatusOK)
	on := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+plain.Id,
		`{"kind":"water","interval_days":7,"moisture_check":true,"water_at":4}`), http.StatusOK)
	off := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+soil.Id, `{"kind":"water","interval_days":7}`), http.StatusOK)
	if fixed.Anchor != apigen.Fixed || on.Anchor != apigen.FromCompletion || !on.MoistureCheck || on.WaterAt != 4 || off.MoistureCheck || off.WaterAt != 3 {
		t.Errorf("fixed = %+v, on = %+v, off = %+v", fixed, on, off)
	}
}
