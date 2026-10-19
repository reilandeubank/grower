package schedule

import "slices"

const (
	// maxCheckDays caps how far ahead a check is scheduled, so a slow estimate or an odd
	// reading never leaves the soil unchecked for long.
	maxCheckDays = 14
	// recentCycles is how many of the latest observed cycles the drying time follows.
	recentCycles = 3
	// wet is the top of the meter's scale.
	wet = 10
)

// DryingDays is how many days the soil takes to dry out after watering, learned from the
// event log (PLAN §5.5): the median of the last three observed cycles, the lower one for
// an even count. ok is false for a task without soil checks, or until a cycle has been
// observed. events are the task's non-voided events in insertion order.
func DryingDays(t Task, events []Event) (days int, ok bool) {
	if !t.MoistureCheck {
		return 0, false
	}
	return dryingDays(t, sortEvents(t, events))
}

func dryingDays(t Task, sorted []Event) (int, bool) {
	var observed []int
	for i, e := range sorted {
		if e.Kind != Done {
			continue
		}
		if on, ok := cycleEnd(t, sorted[i+1:]); ok {
			if d := on.Sub(e.On); d >= 1 {
				observed = append(observed, d)
			}
		}
	}
	if len(observed) == 0 {
		return 0, false
	}
	recent := observed[max(0, len(observed)-recentCycles):]
	slices.Sort(recent)
	return recent[(len(recent)-1)/2], true
}

// cycleEnd is when the soil was seen to be dry after a watering, given the events that
// follow it: the first dry check, else the next watering, whose own reading (taken just
// before) or, without one, the watering itself says it was time. ok is false while the
// cycle is still open.
func cycleEnd(t Task, after []Event) (Date, bool) {
	for _, e := range after {
		if e.Kind == Done || (e.Kind == Checked && e.Reading <= t.WaterAt) {
			return e.On, true
		}
	}
	return Date{}, false
}

// learned is the drying time if one has been observed, else the task's interval: the
// first estimate.
func learned(t Task, sorted []Event) int {
	if d, ok := dryingDays(t, sorted); ok {
		return d
	}
	return t.IntervalDays
}

// nextCheck is a moisture-check task's next check before any snooze (PLAN §5.5). It
// counts from the latest done, skip or check, whatever the anchor or season.
func nextCheck(t Task, sorted []Event) Date {
	i := len(sorted) - 1
	for i >= 0 && sorted[i].Kind == Snoozed {
		i--
	}
	if i < 0 {
		return t.StartOn
	}
	last := sorted[i]
	if last.Kind != Checked {
		return last.On.AddDays(learned(t, sorted))
	}
	c, r := last.On, last.Reading
	if r <= t.WaterAt {
		return c.AddDays(1) // dry but not watered yet: ask again tomorrow
	}
	w, watered := lastWatering(sorted[:i])
	var remaining int
	switch {
	case watered && c.After(w) && r < wet:
		// The soil lost wet − r points in c − w days. At that rate it reaches WaterAt in
		// (r − WaterAt) × (c − w) / (wet − r) days, rounded up so the check isn't early.
		n, per := (r-t.WaterAt)*c.Sub(w), wet-r
		remaining = (n + per - 1) / per
	case watered:
		remaining = learned(t, sorted) - c.Sub(w)
	default:
		remaining = 2
	}
	return c.AddDays(min(max(remaining, 1), maxCheckDays))
}

// lastWatering is the date of the latest done in sorted.
func lastWatering(sorted []Event) (Date, bool) {
	for i := len(sorted) - 1; i >= 0; i-- {
		if sorted[i].Kind == Done {
			return sorted[i].On, true
		}
	}
	return Date{}, false
}

// LastReading is the latest event that carries a meter reading: by date, then by
// insertion order within a day.
func LastReading(events []Event) (Event, bool) {
	var last Event
	found := false
	for _, e := range events {
		if e.Reading > 0 && (!found || !e.On.Before(last.On)) {
			last, found = e, true
		}
	}
	return last, found
}
