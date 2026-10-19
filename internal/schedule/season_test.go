package schedule

import (
	"math/rand/v2"
	"testing"
	"time"
)

func TestSeasonalCurves(t *testing.T) {
	for m := time.January; m <= time.December; m++ {
		for _, kind := range []string{"water", "mist", "fertilize", "repot", "rotate", "custom"} {
			if north, south := percent(kind, m, North), percent(kind, (m+5)%12+1, South); north != south {
				t.Errorf("%s: north %s = %d, south six months on = %d", kind, m, north, south)
			}
		}
		if p := percent("repot", m, South); p != 100 {
			t.Errorf("repot in %s = %d, want 100", m, p)
		}
	}
	tests := []struct {
		kind string
		m    time.Month
		h    Hemisphere
		want int
	}{
		{"water", time.January, North, 150},
		{"water", time.July, North, 85},
		{"water", time.July, South, 150},
		{"water", time.January, South, 85},
		{"water", time.July, "", 85}, // the zero value is north
		{"mist", time.November, North, 140},
		{"fertilize", time.December, North, 200},
		{"fertilize", time.June, South, 200},
		{"fertilize", time.September, North, 125},
		{"custom", time.January, North, 100},
	}
	for _, tt := range tests {
		if got := percent(tt.kind, tt.m, tt.h); got != tt.want {
			t.Errorf("percent(%s, %s, %q) = %d, want %d", tt.kind, tt.m, tt.h, got, tt.want)
		}
	}
}

func TestSeasonalNextDue(t *testing.T) {
	water := func(interval int) Task {
		return Task{IntervalDays: interval, StartOn: d("2026-01-01"), Kind: "water", Seasonal: true}
	}
	tests := []struct {
		name   string
		task   Task
		events []Event
		h      Hemisphere
		want   string
	}{
		{"no completion: start date", water(7), nil, North, "2026-01-01"},
		{"January stretches: 7 × 1.5 rounds half up to 11", water(7), []Event{done("2026-01-10")}, North, "2026-01-21"},
		{"July shortens: 7 × 0.85 = 5.95 → 6", water(7), []Event{done("2026-07-10")}, North, "2026-07-16"},
		{"April is neutral", water(7), []Event{done("2026-04-10")}, North, "2026-04-17"},
		{"south: January is summer", water(7), []Event{done("2026-01-10")}, South, "2026-01-16"},
		{"south: July is winter", water(7), []Event{done("2026-07-10")}, South, "2026-07-21"},
		{"rounding: 10 × 0.85 = 8.5 → 9", water(10), []Event{done("2026-07-01")}, North, "2026-07-10"},
		{"floor: 1 × 0.85 stays 1", water(1), []Event{done("2026-07-01")}, North, "2026-07-02"},
		{"month of last: done Jan 31 uses January", water(10), []Event{done("2026-01-31")}, North, "2026-02-15"},
		{"month of last: done Feb 1 uses February", water(10), []Event{done("2026-02-01")}, North, "2026-02-15"},
		{"skip counts from its month too", water(10), []Event{skipped("2026-12-05")}, North, "2026-12-20"},
		{"snooze still wins when later", water(7), []Event{done("2026-07-10"), snoozed("2026-07-12", "2026-07-20")}, North, "2026-07-20"},
		{"fertilize: winter doubles", Task{IntervalDays: 14, StartOn: d("2026-01-01"), Kind: "fertilize", Seasonal: true}, []Event{done("2026-12-01")}, North, "2026-12-29"},
		{"other kinds keep their interval", Task{IntervalDays: 7, StartOn: d("2026-01-01"), Kind: "rotate", Seasonal: true}, []Event{done("2026-01-10")}, North, "2026-01-17"},
		{"seasonal off", Task{IntervalDays: 7, StartOn: d("2026-01-01"), Kind: "water"}, []Event{done("2026-01-10")}, North, "2026-01-17"},
		{"fixed anchor keeps its calendar", Task{IntervalDays: 7, Anchor: Fixed, StartOn: d("2026-01-01"), Kind: "water", Seasonal: true}, []Event{done("2026-01-10")}, North, "2026-01-15"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextDue(tt.task, tt.events, tt.h); got.String() != tt.want {
				t.Errorf("NextDue = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCurrentInterval(t *testing.T) {
	water := Task{IntervalDays: 7, Kind: "water", Seasonal: true}
	tests := []struct {
		name  string
		task  Task
		today string
		h     Hemisphere
		want  int
	}{
		{"winter", water, "2026-01-15", North, 11},
		{"summer", water, "2026-07-15", North, 6},
		{"south flips it", water, "2026-07-15", South, 11},
		{"not seasonal", Task{IntervalDays: 7, Kind: "water"}, "2026-01-15", North, 7},
		{"fixed anchor", Task{IntervalDays: 7, Anchor: Fixed, Kind: "water", Seasonal: true}, "2026-01-15", North, 7},
	}
	for _, tt := range tests {
		if got := CurrentInterval(tt.task, nil, d(tt.today), tt.h); got != tt.want {
			t.Errorf("%s: CurrentInterval = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// TestSeasonalProperties: turning seasons off never changes a due date, whatever the
// kind or hemisphere, and a seasonal due date is still after the last completion.
func TestSeasonalProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	base := d("2026-01-01")
	day := func() Date { return base.AddDays(r.IntN(400)) }
	kinds := []string{"water", "mist", "fertilize", "repot", "rotate", "custom"}
	for range 2000 {
		task := Task{IntervalDays: 1 + r.IntN(60), Anchor: []Anchor{FromCompletion, Fixed}[r.IntN(2)], StartOn: day(), Kind: kinds[r.IntN(len(kinds))]}
		var events []Event
		for range r.IntN(6) {
			e := Event{Kind: []EventKind{Done, Skipped, Snoozed}[r.IntN(3)], On: day()}
			if e.Kind == Snoozed {
				e.SnoozeUntil = e.On.AddDays(1 + r.IntN(14))
			}
			events = append(events, e)
		}
		plain := Task{IntervalDays: task.IntervalDays, Anchor: task.Anchor, StartOn: task.StartOn}
		want := NextDue(plain, events, North)
		for _, h := range []Hemisphere{North, South} {
			if got := NextDue(task, events, h); got != want {
				t.Fatalf("seasonal off changed %s to %s for %+v %+v", want, got, task, events)
			}
		}
		task.Seasonal = true
		due := NextDue(task, events, []Hemisphere{North, South}[r.IntN(2)])
		for _, e := range events {
			if e.Kind != Snoozed && !due.After(e.On) {
				t.Fatalf("due %s not after %s for %+v %+v", due, e.On, task, events)
			}
		}
	}
}
