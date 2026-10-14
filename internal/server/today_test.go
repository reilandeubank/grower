package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

func (e *env) today(c *http.Cookie) apigen.Today {
	e.t.Helper()
	return expectJSON[apigen.Today](e.t, e.call(c, http.MethodGet, "/api/v1/today", ""), http.StatusOK)
}

// summary lists a Today section as "plant/kind:days_overdue".
func summary(items []apigen.TodayItem) string {
	var out []string
	for _, it := range items {
		out = append(out, fmt.Sprintf("%s/%s:%d", it.Plant.Name, it.Task.Kind, it.DaysOverdue))
	}
	return strings.Join(out, " ")
}

func TestTodayLoop(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	task := e.newTask(c, p.Id, `{"kind":"water","interval_days":3}`)

	check := func(step, overdue, due, upcoming string) {
		t.Helper()
		got := e.today(c)
		if s := summary(got.Overdue) + "|" + summary(got.Due) + "|" + summary(got.Upcoming); s != overdue+"|"+due+"|"+upcoming {
			t.Errorf("%s: today = %s", step, s)
		}
	}
	got := e.today(c)
	if got.Date.String() != "2026-05-01" || len(got.Due) != 1 || got.Due[0].Task.Id != task.Id || got.Due[0].Plant.Id != p.Id {
		t.Fatalf("today = %+v", got)
	}

	ev := e.logEvent(c, task.Id, `{"kind":"done"}`).Event
	check("done", "", "", "Fern/water:-3")
	e.call(c, http.MethodDelete, "/api/v1/events/"+ev.Id, "")
	check("undone", "", "Fern/water:0", "")

	e.clock.t = e.clock.t.AddDate(0, 0, 3)
	check("three days later", "Fern/water:3", "", "")
	e.logEvent(c, task.Id, `{"kind":"snoozed","snooze_until":"2026-05-06"}`)
	check("snoozed two days", "", "", "Fern/water:-2")
	e.logEvent(c, task.Id, `{"kind":"snoozed","snooze_until":"2026-05-20"}`)
	check("snoozed past the week", "", "", "")
	e.clock.t = e.clock.t.AddDate(0, 0, 16)
	check("snooze date reached", "", "Fern/water:0", "")
	e.logEvent(c, task.Id, `{"kind":"skipped"}`)
	check("skipped", "", "", "Fern/water:-3")
}

func TestTodaySections(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	// Today is 2026-05-01.
	basil, aloe, cactus := e.newPlant(c, "basil"), e.newPlant(c, "Aloe"), e.newPlant(c, "Cactus")
	for _, tt := range []struct{ plant, body string }{
		{basil.Id, `{"kind":"water","interval_days":7,"start_on":"2026-04-29"}`},
		{aloe.Id, `{"kind":"water","interval_days":7,"start_on":"2026-04-29"}`},
		{aloe.Id, `{"kind":"mist","interval_days":7,"start_on":"2026-04-21"}`},
		{basil.Id, `{"kind":"rotate","interval_days":7,"start_on":"2026-05-01"}`},
		{aloe.Id, `{"kind":"repot","interval_days":365,"start_on":"2026-05-08"}`},
		{basil.Id, `{"kind":"fertilize","interval_days":7,"start_on":"2026-05-03"}`},
		{aloe.Id, `{"kind":"fertilize","interval_days":7,"start_on":"2026-05-09"}`},
		{cactus.Id, `{"kind":"water","interval_days":7,"start_on":"2026-04-01"}`},
	} {
		e.newTask(c, tt.plant, tt.body)
	}
	// Archived plants drop out of Today entirely.
	e.call(c, http.MethodPut, "/api/v1/plants/"+cactus.Id, `{"name":"Cactus","archived":true}`)

	got := e.today(c)
	if s := summary(got.Overdue); s != "Aloe/mist:10 Aloe/water:2 basil/water:2" {
		t.Errorf("overdue = %s", s)
	}
	if s := summary(got.Due); s != "basil/rotate:0" {
		t.Errorf("due = %s", s)
	}
	if s := summary(got.Upcoming); s != "basil/fertilize:-2 Aloe/repot:-7" {
		t.Errorf("upcoming = %s", s)
	}
	// Counts tasks beyond the week too (Aloe/fertilize), but not the archived cactus's.
	if got.TaskCount != 7 {
		t.Errorf("task_count = %d, want 7", got.TaskCount)
	}
}

func TestTodayFixedAnchor(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	// Slots: 04-01, 04-15, 04-29, 05-13. The first was missed a month ago.
	task := e.newTask(c, p.Id, `{"kind":"fertilize","interval_days":14,"anchor":"fixed","start_on":"2026-04-01"}`)
	if s := summary(e.today(c).Overdue); s != "Fern/fertilize:30" {
		t.Errorf("overdue = %s", s)
	}
	// Missed periods don't stack: one done moves it to the next slot.
	if next := e.logEvent(c, task.Id, `{"kind":"done"}`).Task.NextDue.String(); next != "2026-05-13" {
		t.Errorf("next due = %s", next)
	}
}

func TestTodayUsesUserTimeZone(t *testing.T) {
	// 03:00 UTC on May 1 is still April 30 in Chicago.
	for tz, want := range map[string]string{"America/Chicago": "2026-04-30", "Europe/London": "2026-05-01"} {
		e := newEnv(t, envOptions{env: map[string]string{"GROWER_TZ": tz}})
		e.clock.t = time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
		c := e.loginCookie()
		task := e.newTask(c, e.newPlant(c, "Fern").Id, `{"kind":"water","interval_days":3}`)
		got := e.today(c)
		if got.Date.String() != want || task.StartOn.String() != want || len(got.Due) != 1 {
			t.Errorf("%s: today = %+v, start_on = %s", tz, got, task.StartOn)
		}
		if ev := e.logEvent(c, task.Id, `{"kind":"done"}`).Event; ev.LocalDate.String() != want {
			t.Errorf("%s: local_date = %s", tz, ev.LocalDate)
		}
	}
}
