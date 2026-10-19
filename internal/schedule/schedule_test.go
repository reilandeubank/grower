package schedule

import (
	"math/rand/v2"
	"testing"
	"time"
)

func d(s string) Date {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func done(on string) Event    { return Event{Kind: Done, On: d(on)} }
func skipped(on string) Event { return Event{Kind: Skipped, On: d(on)} }
func snoozed(on, until string) Event {
	return Event{Kind: Snoozed, On: d(on), SnoozeUntil: d(until)}
}

func TestNextDue(t *testing.T) {
	weekly := Task{IntervalDays: 7, Anchor: FromCompletion, StartOn: d("2026-05-01")}
	fixed := Task{IntervalDays: 7, Anchor: Fixed, StartOn: d("2026-05-01")}
	tests := []struct {
		name   string
		task   Task
		events []Event
		want   string
	}{
		{"no events: start in the past", weekly, nil, "2026-05-01"},
		{"no events: start in the future", Task{IntervalDays: 3, StartOn: d("2027-01-01")}, nil, "2027-01-01"},
		{"zero anchor means from completion", Task{IntervalDays: 3, StartOn: d("2026-05-01")}, []Event{done("2026-05-02")}, "2026-05-05"},

		{"from completion: done on time", weekly, []Event{done("2026-05-01")}, "2026-05-08"},
		{"from completion: done early", weekly, []Event{done("2026-04-28")}, "2026-05-05"},
		{"from completion: done late restarts from completion", weekly, []Event{done("2026-05-10")}, "2026-05-17"},
		{"fixed: done on time", fixed, []Event{done("2026-05-01")}, "2026-05-08"},
		{"fixed: done early keeps the slot", fixed, []Event{done("2026-04-29")}, "2026-05-01"},
		{"fixed: done late keeps the calendar", fixed, []Event{done("2026-05-03")}, "2026-05-08"},
		{"fixed: done on a later slot day", fixed, []Event{done("2026-05-08")}, "2026-05-15"},
		{"fixed: several missed periods give the next slot once", fixed, []Event{done("2026-06-02")}, "2026-06-05"},

		{"skip schedules like done", weekly, []Event{skipped("2026-05-02")}, "2026-05-09"},
		{"skip on fixed", fixed, []Event{skipped("2026-05-01")}, "2026-05-08"},
		{"two dones the same day", weekly, []Event{done("2026-05-01"), done("2026-05-01")}, "2026-05-08"},
		{"backdated event inserted out of order", weekly, []Event{done("2026-05-10"), done("2026-05-03")}, "2026-05-17"},

		{"snooze before due pushes later", weekly, []Event{done("2026-05-01"), snoozed("2026-05-07", "2026-05-11")}, "2026-05-11"},
		{"snooze to before due has no effect", weekly, []Event{done("2026-05-01"), snoozed("2026-05-02", "2026-05-04")}, "2026-05-08"},
		{"snooze with no completions", weekly, []Event{snoozed("2026-04-30", "2026-05-03")}, "2026-05-03"},
		{"snooze while overdue", weekly, []Event{done("2026-05-01"), snoozed("2026-05-12", "2026-05-14")}, "2026-05-14"},
		{"latest snooze wins over an earlier one", weekly, []Event{snoozed("2026-04-30", "2026-05-09"), snoozed("2026-05-02", "2026-05-04")}, "2026-05-04"},
		{"snooze then done: done wins", weekly, []Event{snoozed("2026-05-01", "2026-05-05"), done("2026-05-02")}, "2026-05-09"},
		{"snooze then done the same day: done wins", weekly, []Event{snoozed("2026-05-01", "2026-05-20"), done("2026-05-01")}, "2026-05-08"},
		{"done then snooze the same day: snooze counts", weekly, []Event{done("2026-05-01"), snoozed("2026-05-01", "2026-05-12")}, "2026-05-12"},
		{"snooze backdated before the last done is ignored", weekly, []Event{done("2026-05-05"), snoozed("2026-05-04", "2026-05-20")}, "2026-05-12"},
		{"fixed: snooze does not move the anchor", fixed, []Event{done("2026-05-01"), snoozed("2026-05-07", "2026-05-10"), done("2026-05-10")}, "2026-05-15"},

		{"leap day", Task{IntervalDays: 1, StartOn: d("2028-02-01")}, []Event{done("2028-02-28")}, "2028-02-29"},
		{"no leap day in 2026", Task{IntervalDays: 1, StartOn: d("2026-02-01")}, []Event{done("2026-02-28")}, "2026-03-01"},
		{"year boundary", Task{IntervalDays: 3, StartOn: d("2026-12-01")}, []Event{done("2026-12-30")}, "2027-01-02"},
		{"fixed across a year boundary", Task{IntervalDays: 30, Anchor: Fixed, StartOn: d("2026-11-15")}, []Event{done("2026-12-20")}, "2027-01-14"},
		{"interval 1", Task{IntervalDays: 1, Anchor: Fixed, StartOn: d("2026-05-01")}, []Event{done("2026-05-04")}, "2026-05-05"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextDue(tt.task, tt.events, North); got.String() != tt.want {
				t.Errorf("NextDue = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNextDueDoesNotReorderInput(t *testing.T) {
	events := []Event{done("2026-05-10"), done("2026-05-03")}
	NextDue(Task{IntervalDays: 7, StartOn: d("2026-05-01")}, events, North)
	if events[0].On != d("2026-05-10") {
		t.Error("NextDue sorted the caller's slice")
	}
}

func TestLastDone(t *testing.T) {
	if _, ok := LastDone(nil); ok {
		t.Error("LastDone(nil) reported a date")
	}
	if _, ok := LastDone([]Event{skipped("2026-05-01"), snoozed("2026-05-01", "2026-05-03")}); ok {
		t.Error("skips and snoozes count as done")
	}
	got, ok := LastDone([]Event{done("2026-05-04"), done("2026-05-02"), skipped("2026-05-06")})
	if !ok || got != d("2026-05-04") {
		t.Errorf("LastDone = %s, %v", got, ok)
	}
}

// TestNextDueProperties checks invariants over random tasks and event logs.
func TestNextDueProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	base := d("2026-01-01")
	day := func() Date { return base.AddDays(r.IntN(400)) }
	for range 2000 {
		task := Task{IntervalDays: 1 + r.IntN(60), Anchor: []Anchor{FromCompletion, Fixed}[r.IntN(2)], StartOn: day()}
		var events []Event
		for range r.IntN(6) {
			e := Event{Kind: []EventKind{Done, Skipped, Snoozed}[r.IntN(3)], On: day()}
			if e.Kind == Snoozed {
				e.SnoozeUntil = e.On.AddDays(1 + r.IntN(14))
			}
			events = append(events, e)
		}
		due := NextDue(task, events, North)
		if again := NextDue(task, events, North); again != due {
			t.Fatalf("not deterministic: %s then %s for %+v %+v", due, again, task, events)
		}
		var last Date
		for _, e := range events {
			if e.Kind != Snoozed && e.On.After(last) {
				last = e.On
			}
		}
		if last != (Date{}) && !due.After(last) {
			t.Fatalf("due %s not after last %s for %+v %+v", due, last, task, events)
		}
	}
}

func TestDate(t *testing.T) {
	for _, s := range []string{"", "2026-02-30", "2026-5-1", "2026-05-01T00:00:00Z"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) succeeded", s)
		}
	}
	a, b := d("2026-12-31"), d("2027-03-01")
	if a.AddDays(1).String() != "2027-01-01" || b.AddDays(-1).String() != "2027-02-28" {
		t.Error("AddDays across month or year")
	}
	if b.Sub(a) != 60 || a.Sub(b) != -60 || a.Sub(a) != 0 {
		t.Errorf("Sub = %d", b.Sub(a))
	}
	if !a.Before(b) || a.After(b) || !b.After(a) || a.Compare(b) != -1 || a.Compare(a) != 0 {
		t.Error("ordering")
	}
	// A whole year containing a DST change is still 365 days.
	if got := d("2027-01-01").Sub(d("2026-01-01")); got != 365 {
		t.Errorf("year = %d days", got)
	}
}

func TestToday(t *testing.T) {
	load := func(name string) *time.Location {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		return loc
	}
	chicago, london, sydney := load("America/Chicago"), load("Europe/London"), load("Australia/Sydney")
	tests := []struct {
		name string
		now  string // a UTC instant
		loc  *time.Location
		want string
	}{
		{"Chicago 23:59 before spring forward", "2026-03-08T05:59:00Z", chicago, "2026-03-07"},
		{"Chicago 00:01 on spring forward day", "2026-03-08T06:01:00Z", chicago, "2026-03-08"},
		{"Chicago 23:59 on spring forward day", "2026-03-09T04:59:00Z", chicago, "2026-03-08"},
		{"Chicago 00:01 after spring forward", "2026-03-09T05:01:00Z", chicago, "2026-03-09"},
		{"Chicago 00:01 on fall back day", "2026-11-01T05:01:00Z", chicago, "2026-11-01"},
		{"Chicago 23:59 on fall back day", "2026-11-02T05:59:00Z", chicago, "2026-11-01"},
		{"Chicago 00:01 after fall back", "2026-11-02T06:01:00Z", chicago, "2026-11-02"},
		{"London 00:01 on spring forward day", "2026-03-29T00:01:00Z", london, "2026-03-29"},
		{"London 23:59 on spring forward day", "2026-03-29T22:59:00Z", london, "2026-03-29"},
		{"London 00:01 on fall back day", "2026-10-24T23:01:00Z", london, "2026-10-25"},
		{"London 23:59 on fall back day", "2026-10-25T23:59:00Z", london, "2026-10-25"},
		{"Sydney 23:59 before fall back", "2026-04-04T12:59:00Z", sydney, "2026-04-04"},
		{"Sydney 00:01 on fall back day", "2026-04-04T13:01:00Z", sydney, "2026-04-05"},
		{"Sydney 23:59 on fall back day", "2026-04-05T13:59:00Z", sydney, "2026-04-05"},
		{"Sydney 00:01 on spring forward day", "2026-10-03T14:01:00Z", sydney, "2026-10-04"},
		{"Sydney 23:59 on spring forward day", "2026-10-04T12:59:00Z", sydney, "2026-10-04"},
		{"same instant, other zone", "2026-05-01T03:00:00Z", chicago, "2026-04-30"},
		{"same instant, UTC", "2026-05-01T03:00:00Z", time.UTC, "2026-05-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			if got := Today(now, tt.loc); got.String() != tt.want {
				t.Errorf("Today = %s, want %s (local %s)", got, tt.want, now.In(tt.loc))
			}
		})
	}
}
