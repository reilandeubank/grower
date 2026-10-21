package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
)

// newPlant creates a plant and returns it.
func (e *env) newPlant(c *http.Cookie, name string) apigen.Plant {
	e.t.Helper()
	return expectJSON[apigen.Plant](e.t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"`+name+`"}`), http.StatusCreated)
}

// newTask creates a task on a plant and returns it.
func (e *env) newTask(c *http.Cookie, plantID, body string) apigen.CareTask {
	e.t.Helper()
	return expectJSON[apigen.CareTask](e.t, e.call(c, http.MethodPost, "/api/v1/plants/"+plantID+"/tasks", body), http.StatusCreated)
}

// logEvent records an event and returns the event and recomputed task.
func (e *env) logEvent(c *http.Cookie, taskID, body string) apigen.LoggedEvent {
	e.t.Helper()
	return expectJSON[apigen.LoggedEvent](e.t, e.call(c, http.MethodPost, "/api/v1/tasks/"+taskID+"/events", body), http.StatusCreated)
}

func nextAndLast(t *apigen.CareTask) (next, last string) {
	if t.LastDone != nil {
		last = t.LastDone.String()
	}
	return t.NextDue.String(), last
}

func TestTaskCRUD(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")

	water := e.newTask(c, p.Id, `{"kind":"water","label":"ignored","interval_days":3}`)
	if water.Id == "" || water.PlantId != p.Id || water.Kind != apigen.Water || water.Label != "" || water.IntervalDays != 3 ||
		water.Anchor != apigen.FromCompletion || water.StartOn.String() != "2026-05-01" || water.NextDue.String() != "2026-05-01" ||
		water.LastDone != nil || !water.Seasonal || water.CurrentIntervalDays != 3 {
		t.Errorf("water = %+v", water)
	}
	custom := e.newTask(c, p.Id, `{"kind":"custom","label":"  Wipe leaves ","interval_days":14,"anchor":"fixed","start_on":"2026-04-20"}`)
	if custom.Label != "Wipe leaves" || custom.Anchor != apigen.Fixed || custom.NextDue.String() != "2026-04-20" || custom.Seasonal {
		t.Errorf("custom = %+v", custom)
	}
	mist := e.newTask(c, p.Id, `{"kind":"mist","interval_days":2,"start_on":"2026-05-01"}`)

	list := expectJSON[apigen.CareTaskList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/tasks", ""), http.StatusOK)
	var ids []string
	for _, it := range list.Items {
		ids = append(ids, it.Id)
	}
	// By next due date, then kind: custom (04-20), then mist and water (05-01).
	if strings.Join(ids, ",") != custom.Id+","+mist.Id+","+water.Id {
		t.Errorf("order = %v", list.Items)
	}

	// Updating the schedule keeps the history and recomputes from it.
	e.logEvent(c, water.Id, `{"kind":"done"}`)
	updated := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+water.Id,
		`{"kind":"water","interval_days":10}`), http.StatusOK)
	if next, last := nextAndLast(&updated); next != "2026-05-11" || last != "2026-05-01" {
		t.Errorf("updated next %s, last %s", next, last)
	}

	if resp := e.call(c, http.MethodDelete, "/api/v1/tasks/"+water.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	expectProblem(t, e.call(c, http.MethodDelete, "/api/v1/tasks/"+water.Id, ""), http.StatusNotFound)
	history := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events", ""), http.StatusOK)
	if len(history.Items) != 0 {
		t.Errorf("deleted task left history: %+v", history.Items)
	}

	// Deleting the plant deletes its tasks.
	e.call(c, http.MethodDelete, "/api/v1/plants/"+p.Id, "")
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/tasks/"+mist.Id, `{"kind":"mist","interval_days":2}`), http.StatusNotFound)
}

func TestTaskEvents(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	task := e.newTask(c, p.Id, `{"kind":"water","interval_days":3}`)

	logged := e.logEvent(c, task.Id, `{"kind":"done","note":"Soaked"}`)
	ev := logged.Event
	if ev.Id == "" || ev.TaskId != task.Id || ev.Kind != apigen.Done || ev.LocalDate.String() != "2026-05-01" ||
		ev.SnoozeUntil != nil || ev.Note != "Soaked" || !ev.OccurredAt.Equal(e.clock.t) {
		t.Errorf("event = %+v", ev)
	}
	if next, last := nextAndLast(&logged.Task); next != "2026-05-04" || last != "2026-05-01" {
		t.Errorf("after done: next %s, last %s", next, last)
	}

	undone := expectJSON[apigen.CareTask](t, e.call(c, http.MethodDelete, "/api/v1/events/"+ev.Id, ""), http.StatusOK)
	if next, last := nextAndLast(&undone); next != "2026-05-01" || last != "" {
		t.Errorf("after undo: next %s, last %s", next, last)
	}
	// Undoing again is harmless.
	expectJSON[apigen.CareTask](t, e.call(c, http.MethodDelete, "/api/v1/events/"+ev.Id, ""), http.StatusOK)

	// Yesterday is allowed (a tap just before midnight); skips schedule like done.
	skip := e.logEvent(c, task.Id, `{"kind":"skipped","local_date":"2026-04-30"}`)
	if next, last := nextAndLast(&skip.Task); next != "2026-05-03" || last != "" {
		t.Errorf("after skip: next %s, last %s", next, last)
	}
	snooze := e.logEvent(c, task.Id, `{"kind":"snoozed","snooze_until":"2026-05-06"}`)
	if snooze.Event.SnoozeUntil == nil || snooze.Event.SnoozeUntil.String() != "2026-05-06" || snooze.Task.NextDue.String() != "2026-05-06" {
		t.Errorf("snooze = %+v", snooze)
	}
	e.logEvent(c, task.Id, `{"kind":"done"}`)

	history := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events", ""), http.StatusOK)
	var kinds []string
	for _, h := range history.Items {
		if h.TaskKind != apigen.Water || h.TaskLabel != "" {
			t.Errorf("entry = %+v", h)
		}
		kinds = append(kinds, string(h.Event.Kind)+"@"+h.Event.LocalDate.String())
	}
	// Newest date first, then newest logged; the undone event is left out.
	if got := strings.Join(kinds, " "); got != "done@2026-05-01 snoozed@2026-05-01 skipped@2026-04-30" {
		t.Errorf("history = %s", got)
	}
	limited := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events?limit=1", ""), http.StatusOK)
	if len(limited.Items) != 1 || limited.Items[0].Event.Kind != apigen.Done {
		t.Errorf("limit=1 = %+v", limited.Items)
	}
}

func TestCareValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	task := e.newTask(c, p.Id, `{"kind":"water","interval_days":3}`)
	tasks, events := "/api/v1/plants/"+p.Id+"/tasks", "/api/v1/tasks/"+task.Id+"/events"

	tests := []struct {
		name, method, path, body string
		status                   int
		detail                   string
	}{
		{"bad kind", http.MethodPost, tasks, `{"kind":"prune","interval_days":3}`, 400, "Kind must be water, fertilize, mist, repot, rotate, custom or check_in."},
		{"custom without label", http.MethodPost, tasks, `{"kind":"custom","label":"  ","interval_days":3}`, 400, "Label is required for a custom task."},
		{"label too long", http.MethodPost, tasks, `{"kind":"custom","label":"` + strings.Repeat("é", 41) + `","interval_days":3}`, 400, "Label must be 40 characters or fewer."},
		{"interval 0", http.MethodPost, tasks, `{"kind":"water","interval_days":0}`, 400, "Interval must be 1 to 365 days."},
		{"interval 366", http.MethodPut, "/api/v1/tasks/" + task.Id, `{"kind":"water","interval_days":366}`, 400, "Interval must be 1 to 365 days."},
		{"bad anchor", http.MethodPost, tasks, `{"kind":"water","interval_days":3,"anchor":"monthly"}`, 400, "Anchor must be from_completion or fixed."},
		{"bad start_on", http.MethodPost, tasks, `{"kind":"water","interval_days":3,"start_on":"2026-02-30"}`, 400, "The request body is not valid JSON for this endpoint."},
		{"task on unknown plant", http.MethodPost, "/api/v1/plants/nope/tasks", `{"kind":"water","interval_days":3}`, 404, "Plant not found."},
		{"tasks of unknown plant", http.MethodGet, "/api/v1/plants/nope/tasks", "", 404, "Plant not found."},
		{"history of unknown plant", http.MethodGet, "/api/v1/plants/nope/events", "", 404, "Plant not found."},
		{"update unknown task", http.MethodPut, "/api/v1/tasks/nope", `{"kind":"water","interval_days":3}`, 404, "Task not found."},
		{"delete unknown task", http.MethodDelete, "/api/v1/tasks/nope", "", 404, "Task not found."},

		{"bad event kind", http.MethodPost, events, `{"kind":"watered"}`, 400, "Kind must be done, skipped, snoozed or checked."},
		{"two days ago", http.MethodPost, events, `{"kind":"done","local_date":"2026-04-29"}`, 400, "That date is too far from today."},
		{"two days ahead", http.MethodPost, events, `{"kind":"done","local_date":"2026-05-03"}`, 400, "That date is too far from today."},
		{"snooze without date", http.MethodPost, events, `{"kind":"snoozed"}`, 400, "Pick a snooze date after today."},
		{"snooze to today", http.MethodPost, events, `{"kind":"snoozed","snooze_until":"2026-05-01"}`, 400, "Pick a snooze date after today."},
		{"snooze into the past", http.MethodPost, events, `{"kind":"snoozed","snooze_until":"2026-04-01"}`, 400, "Pick a snooze date after today."},
		{"snooze date on done", http.MethodPost, events, `{"kind":"done","snooze_until":"2026-05-03"}`, 400, "Only a snooze takes a snooze date."},
		{"note too long", http.MethodPost, events, `{"kind":"done","note":"` + strings.Repeat("a", 501) + `"}`, 400, "Note must be 500 characters or fewer."},
		{"event on unknown task", http.MethodPost, "/api/v1/tasks/nope/events", `{"kind":"done"}`, 404, "Task not found."},
		{"undo unknown event", http.MethodDelete, "/api/v1/events/nope", "", 404, "Event not found."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectProblem(t, e.call(c, tt.method, tt.path, tt.body), tt.status); got != tt.detail {
				t.Errorf("detail = %q, want %q", got, tt.detail)
			}
		})
	}
	history := expectJSON[apigen.CareEventList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/events", ""), http.StatusOK)
	if len(history.Items) != 0 {
		t.Errorf("rejected events were stored: %+v", history.Items)
	}
}

func TestCareNeedsAuth(t *testing.T) {
	e := newEnv(t, envOptions{})
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/today"},
		{http.MethodGet, "/api/v1/plants/x/tasks"},
		{http.MethodPost, "/api/v1/plants/x/tasks"},
		{http.MethodGet, "/api/v1/plants/x/events"},
		{http.MethodPut, "/api/v1/tasks/x"},
		{http.MethodDelete, "/api/v1/tasks/x"},
		{http.MethodPost, "/api/v1/tasks/x/events"},
		{http.MethodDelete, "/api/v1/events/x"},
	} {
		expectProblem(t, e.call(nil, r.method, r.path, `{}`), http.StatusUnauthorized)
	}
}
