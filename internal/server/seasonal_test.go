package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

// TestSeasonalSchedule: a seasonal task's interval follows the month it was done in and
// the user's hemisphere, in task responses and on Today alike.
func TestSeasonalSchedule(t *testing.T) {
	e := newEnv(t, envOptions{})
	e.clock.t = time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC) // January 15 in Chicago
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	task := e.newTask(c, p.Id, `{"kind":"water","interval_days":7}`)
	setHemisphere := func(h string) {
		t.Helper()
		expectJSON[apigen.Settings](t, e.call(c, http.MethodPut, "/api/v1/settings",
			`{"timezone":"America/Chicago","digest_time":"08:00","hemisphere":"`+h+`"}`), http.StatusOK)
	}
	check := func(step string, got apigen.CareTask, next string, current int, upcoming string) {
		t.Helper()
		if got.NextDue.String() != next || got.CurrentIntervalDays != current || got.IntervalDays != 7 {
			t.Errorf("%s: next %s, current %d, interval %d", step, got.NextDue, got.CurrentIntervalDays, got.IntervalDays)
		}
		if s := summary(e.today(c).Upcoming); s != upcoming {
			t.Errorf("%s: upcoming = %q", step, s)
		}
	}

	// Northern winter: 7 × 1.5 = 10.5, rounded to 11, which is past Today's week.
	check("north, January", e.logEvent(c, task.Id, `{"kind":"done"}`).Task, "2026-01-26", 11, "")
	// Southern summer: 7 × 0.85 = 5.95, rounded to 6.
	setHemisphere("south")
	check("south, January", e.reloadTask(c, task), "2026-01-21", 6, "Fern/water:-6")
	// Turning it off restores the plain interval; PUT without seasonal uses the kind's default.
	off := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+task.Id,
		`{"kind":"water","interval_days":7,"seasonal":false}`), http.StatusOK)
	check("seasonal off", off, "2026-01-22", 7, "Fern/water:-7")
	on := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+task.Id,
		`{"kind":"water","interval_days":7}`), http.StatusOK)
	check("default on again", on, "2026-01-21", 6, "Fern/water:-6")
	// A fixed schedule keeps its calendar.
	fixed := expectJSON[apigen.CareTask](t, e.call(c, http.MethodPut, "/api/v1/tasks/"+task.Id,
		`{"kind":"water","interval_days":7,"anchor":"fixed","start_on":"2026-01-15"}`), http.StatusOK)
	check("fixed", fixed, "2026-01-22", 7, "Fern/water:-7")

	// Northern summer.
	e.call(c, http.MethodPut, "/api/v1/tasks/"+task.Id, `{"kind":"water","interval_days":7}`)
	setHemisphere("north")
	e.clock.t = time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	c = e.loginCookie() // the session expired over six months
	check("north, July", e.logEvent(c, task.Id, `{"kind":"done"}`).Task, "2026-07-21", 6, "Fern/water:-6")
	setHemisphere("south")
	check("south, July", e.reloadTask(c, task), "2026-07-26", 11, "")

	if rotate := e.newTask(c, p.Id, `{"kind":"rotate","interval_days":7}`); rotate.Seasonal {
		t.Errorf("rotate defaulted to seasonal: %+v", rotate)
	}
}

// reloadTask reads a task back through its plant's task list.
func (e *env) reloadTask(c *http.Cookie, task apigen.CareTask) apigen.CareTask {
	e.t.Helper()
	list := expectJSON[apigen.CareTaskList](e.t, e.call(c, http.MethodGet, "/api/v1/plants/"+task.PlantId+"/tasks", ""), http.StatusOK)
	for _, it := range list.Items {
		if it.Id == task.Id {
			return it
		}
	}
	e.t.Fatalf("task %s not listed", task.Id)
	return apigen.CareTask{}
}
