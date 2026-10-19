package schedule

import (
	"math/rand/v2"
	"testing"
)

func checked(on string, reading int) Event { return Event{Kind: Checked, On: d(on), Reading: reading} }

// watered is a done with the reading taken just before watering.
func watered(on string, reading int) Event { return Event{Kind: Done, On: d(on), Reading: reading} }

// soilCheck is a water task checked with a meter: first estimate 7 days, water at 3.
func soilCheck() Task {
	return Task{IntervalDays: 7, StartOn: d("2026-05-01"), Kind: "water", MoistureCheck: true, WaterAt: 3}
}

func TestDryingDays(t *testing.T) {
	tests := []struct {
		name   string
		events []Event
		want   int // 0 = none learned
	}{
		{"no events", nil, 0},
		{"one watering, no observation yet", []Event{done("2026-05-01")}, 0},
		{"moist check is not an observation", []Event{done("2026-05-01"), checked("2026-05-04", 6)}, 0},
		{"one cycle: days to the dry check", []Event{done("2026-05-01"), checked("2026-05-04", 6), checked("2026-05-06", 3)}, 5},
		{"only the first dry check counts", []Event{done("2026-05-01"), checked("2026-05-06", 2), checked("2026-05-08", 1)}, 5},
		{"a done without a reading closes the cycle", []Event{done("2026-05-01"), done("2026-05-07")}, 6},
		{"a done with a moist reading still closes it at its date", []Event{done("2026-05-01"), watered("2026-05-07", 5)}, 6},
		{"a done's dry reading is the observation", []Event{done("2026-05-01"), checked("2026-05-04", 6), watered("2026-05-08", 2)}, 7},
		{"the dry check comes before the next done", []Event{done("2026-05-01"), checked("2026-05-06", 3), done("2026-05-09")}, 5},
		{"a check before any watering is ignored", []Event{checked("2026-04-28", 2), done("2026-05-01")}, 0},
		{"same-day dry check is ignored", []Event{done("2026-05-01"), checked("2026-05-01", 2)}, 0},
		{"two dones the same day give no observation", []Event{done("2026-05-01"), done("2026-05-01")}, 0},
		{"skips and snoozes don't end a cycle", []Event{done("2026-05-01"), skipped("2026-05-05"), snoozed("2026-05-05", "2026-05-07"), checked("2026-05-08", 3)}, 7},
		{"water_at is the threshold", []Event{done("2026-05-01"), checked("2026-05-05", 4), checked("2026-05-07", 3)}, 6},
		{"median of 3", []Event{done("2026-05-01"), done("2026-05-09"), done("2026-05-13"), done("2026-05-19")}, 6},
		{"median of 2 is the lower", []Event{done("2026-05-01"), done("2026-05-09"), done("2026-05-13")}, 4},
		// 1, 6, 7, 8 days: the last three give 7; all four would give 6.
		{"only the last 3 count", []Event{done("2026-04-01"), done("2026-04-02"), done("2026-04-08"), done("2026-04-15"), done("2026-04-23")}, 7},
		{"backdated input is sorted", []Event{checked("2026-05-06", 3), done("2026-05-01")}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DryingDays(soilCheck(), tt.events)
			if ok != (tt.want != 0) || got != tt.want {
				t.Errorf("DryingDays = %d, %v; want %d", got, ok, tt.want)
			}
		})
	}
	waterAt5 := soilCheck()
	waterAt5.WaterAt = 5
	if got, _ := DryingDays(waterAt5, []Event{done("2026-05-01"), checked("2026-05-05", 4)}); got != 4 {
		t.Errorf("water_at 5: DryingDays = %d, want 4", got)
	}
	plain := soilCheck()
	plain.MoistureCheck = false
	if _, ok := DryingDays(plain, []Event{done("2026-05-01"), done("2026-05-05")}); ok {
		t.Error("a task without soil checks learned a drying time")
	}
}

func TestNextCheck(t *testing.T) {
	tests := []struct {
		name   string
		task   Task
		events []Event
		want   string
	}{
		{"no events: start date", soilCheck(), nil, "2026-05-01"},
		{"snooze alone still applies", soilCheck(), []Event{snoozed("2026-05-01", "2026-05-04")}, "2026-05-04"},
		{"done: + interval when nothing learned", soilCheck(), []Event{done("2026-05-01")}, "2026-05-08"},
		{"skip counts like done", soilCheck(), []Event{skipped("2026-05-02")}, "2026-05-09"},
		{"skip after a check", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-04", 6), skipped("2026-05-05")}, "2026-05-12"},
		{"done: + learned drying days", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-06", 3), done("2026-05-06")}, "2026-05-11"},
		{"watered after a dry reading: learned from it", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-04", 6), watered("2026-05-05", 2)}, "2026-05-09"},

		{"dry check not watered: tomorrow", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-06", 2)}, "2026-05-07"},
		{"reading at water_at is dry", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-06", 3)}, "2026-05-07"},
		// 4 points in 3 days; 3 more points to reach 3 → ceil(2.25) = 3 days.
		{"moist: rate estimate rounds up", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-04", 6)}, "2026-05-07"},
		// 1 point in 4 days; 6 to go → 24 days, capped at 14.
		{"moist: slow drying capped at 14", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-05", 9)}, "2026-05-19"},
		// 6 points in 1 day; 1 to go → ceil(1/6) = 1.
		{"moist: fast drying at least 1", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-02", 4)}, "2026-05-03"},
		{"moist: latest check and last watering", soilCheck(), []Event{done("2026-04-20"), done("2026-05-01"), checked("2026-05-03", 8), checked("2026-05-05", 6)}, "2026-05-08"},
		{"still 10: learned minus days since watering", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-03", 10)}, "2026-05-08"},
		{"still 10 past the estimate: tomorrow", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-10", 10)}, "2026-05-11"},
		{"same-day check: learned from watering", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-01", 6)}, "2026-05-08"},
		{"no watering yet: 2 days", soilCheck(), []Event{checked("2026-05-01", 6)}, "2026-05-03"},
		{"long estimate capped at 14", Task{IntervalDays: 30, StartOn: d("2026-05-01"), MoistureCheck: true, WaterAt: 3}, []Event{done("2026-05-01"), checked("2026-05-01", 10)}, "2026-05-15"},

		{"water_at 2: still moist at 3", Task{IntervalDays: 7, StartOn: d("2026-05-01"), MoistureCheck: true, WaterAt: 2}, []Event{done("2026-05-01"), checked("2026-05-04", 3)}, "2026-05-05"},
		{"water_at 4: dry at 4", Task{IntervalDays: 7, StartOn: d("2026-05-01"), MoistureCheck: true, WaterAt: 4}, []Event{done("2026-05-01"), checked("2026-05-04", 4)}, "2026-05-05"},
		// 4 points in 3 days: 4 to go → 3 days at water_at 2, ceil(1.5) = 2 at water_at 4.
		{"water_at 2: later", Task{IntervalDays: 7, StartOn: d("2026-05-01"), MoistureCheck: true, WaterAt: 2}, []Event{done("2026-05-01"), checked("2026-05-04", 6)}, "2026-05-07"},
		{"water_at 4: sooner", Task{IntervalDays: 7, StartOn: d("2026-05-01"), MoistureCheck: true, WaterAt: 4}, []Event{done("2026-05-01"), checked("2026-05-04", 6)}, "2026-05-06"},

		{"snooze after a check pushes later", soilCheck(), []Event{done("2026-05-01"), checked("2026-05-06", 2), snoozed("2026-05-06", "2026-05-09")}, "2026-05-09"},
		{"snooze to before due has no effect", soilCheck(), []Event{done("2026-05-01"), snoozed("2026-05-02", "2026-05-04")}, "2026-05-08"},
		{"check after a snooze clears it", soilCheck(), []Event{done("2026-05-01"), snoozed("2026-05-02", "2026-05-20"), checked("2026-05-06", 2)}, "2026-05-07"},
		{"backdated check inserted out of order", soilCheck(), []Event{checked("2026-05-04", 6), done("2026-05-01")}, "2026-05-07"},
		{"seasons and a fixed anchor are ignored", Task{IntervalDays: 7, Anchor: Fixed, StartOn: d("2026-01-01"), Kind: "water", Seasonal: true, MoistureCheck: true, WaterAt: 3}, []Event{done("2026-01-10")}, "2026-01-17"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextDue(tt.task, tt.events, North); got.String() != tt.want {
				t.Errorf("NextDue = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestNextCheckIgnoresVoided: callers drop undone events, so undoing a check restores
// the due date from before it.
func TestNextCheckIgnoresVoided(t *testing.T) {
	before := []Event{done("2026-05-01")}
	after := append(before[:1:1], checked("2026-05-04", 6))
	if NextDue(soilCheck(), after, North) == NextDue(soilCheck(), before, North) {
		t.Fatal("the check did not move the due date")
	}
	if got := NextDue(soilCheck(), after[:1], North); got.String() != "2026-05-08" {
		t.Errorf("without the voided check: %s", got)
	}
}

func TestChecksDontAffectOtherTasks(t *testing.T) {
	weekly := Task{IntervalDays: 7, StartOn: d("2026-05-01"), Kind: "water"}
	events := []Event{done("2026-05-01"), snoozed("2026-05-03", "2026-05-10"), checked("2026-05-04", 2)}
	if got := NextDue(weekly, events, North); got.String() != "2026-05-10" {
		t.Errorf("NextDue = %s, want the snooze to stand", got)
	}
	if got := CurrentInterval(weekly, events, d("2026-05-04"), North); got != 7 {
		t.Errorf("CurrentInterval = %d", got)
	}
}

func TestCurrentIntervalLearned(t *testing.T) {
	task := soilCheck()
	task.Seasonal = true
	if got := CurrentInterval(task, nil, d("2026-01-15"), North); got != 7 {
		t.Errorf("nothing learned: %d, want interval_days", got)
	}
	if got := CurrentInterval(task, []Event{done("2026-01-01"), checked("2026-01-05", 3)}, d("2026-01-15"), North); got != 4 {
		t.Errorf("learned: %d, want 4", got)
	}
}

func TestLastReading(t *testing.T) {
	if _, ok := LastReading([]Event{done("2026-05-01"), skipped("2026-05-02")}); ok {
		t.Error("events without readings gave one")
	}
	got, ok := LastReading([]Event{checked("2026-05-04", 6), watered("2026-05-06", 2), checked("2026-05-03", 9), done("2026-05-07")})
	if !ok || got.Reading != 2 || got.On != d("2026-05-06") {
		t.Errorf("LastReading = %+v, %v", got, ok)
	}
	got, _ = LastReading([]Event{checked("2026-05-04", 6), checked("2026-05-04", 4)})
	if got.Reading != 4 {
		t.Errorf("same day: %d, want the later logged", got.Reading)
	}
}

// TestNextCheckProperties: the next check is after the latest done, skip or check, and
// within 14 days of a check when one is latest and not snoozed past.
func TestNextCheckProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	base := d("2026-01-01")
	day := func() Date { return base.AddDays(r.IntN(120)) }
	for range 5000 {
		task := Task{IntervalDays: 1 + r.IntN(60), StartOn: day(), Kind: "water", MoistureCheck: true, WaterAt: 1 + r.IntN(9)}
		var events []Event
		for range r.IntN(10) {
			e := Event{Kind: []EventKind{Done, Skipped, Snoozed, Checked, Checked}[r.IntN(5)], On: day()}
			switch e.Kind {
			case Snoozed:
				e.SnoozeUntil = e.On.AddDays(1 + r.IntN(14))
			case Checked:
				e.Reading = 1 + r.IntN(10)
			case Done:
				e.Reading = r.IntN(11) // 0 = no reading
			}
			events = append(events, e)
		}
		due := NextDue(task, events, North)
		if again := NextDue(task, events, North); again != due {
			t.Fatalf("not deterministic: %s then %s", due, again)
		}
		var latest Event
		for _, e := range events {
			if e.Kind != Snoozed && !e.On.Before(latest.On) {
				latest = e
			}
		}
		if latest.Kind == "" {
			continue
		}
		if !due.After(latest.On) {
			t.Fatalf("due %s not after %s for %+v %+v", due, latest.On, task, events)
		}
		if latest.Kind == Checked && due.Sub(latest.On) > 14 && !snoozedTo(events, due) {
			t.Fatalf("due %s more than 14 days after check %s for %+v %+v", due, latest.On, task, events)
		}
	}
}

func snoozedTo(events []Event, due Date) bool {
	for _, e := range events {
		if e.Kind == Snoozed && e.SnoozeUntil == due {
			return true
		}
	}
	return false
}
