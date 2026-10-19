// Package schedule derives care due dates from a task and its event log (PLAN §5).
// It is pure: no clock, no I/O. Callers pass the dates and events.
package schedule

import "slices"

// Anchor decides what the next due date counts from. Values match the API and database.
type Anchor string

const (
	// FromCompletion counts the interval from the last done or skipped date. It is the default.
	FromCompletion Anchor = "from_completion"
	// Fixed keeps due dates on StartOn + k × interval however late or early it was done.
	Fixed Anchor = "fixed"
)

// EventKind is what happened to a task. Values match the API and database.
type EventKind string

const (
	Done    EventKind = "done"
	Skipped EventKind = "skipped"
	Snoozed EventKind = "snoozed"
)

// Task is a recurring care task. IntervalDays must be at least 1. When Seasonal is set,
// Kind (water, mist, ...) picks the curve that adjusts the interval by month.
type Task struct {
	IntervalDays int
	Anchor       Anchor
	StartOn      Date
	Kind         string
	Seasonal     bool
}

// Event is one logged action. SnoozeUntil is set only for snoozes.
type Event struct {
	Kind        EventKind
	On          Date
	SnoozeUntil Date
}

// NextDue returns the task's next due date. events are the task's non-voided events in
// insertion order; ties on the same date keep that order. Missed periods never stack:
// there is one due date, and a task is overdue by today − due. h picks the seasons for a
// seasonal task.
func NextDue(t Task, events []Event, h Hemisphere) Date {
	sorted := slices.Clone(events)
	slices.SortStableFunc(sorted, func(a, b Event) int { return a.On.Compare(b.On) })

	due := t.StartOn
	if i := lastCompletion(sorted); i >= 0 {
		last := sorted[i].On
		switch t.Anchor {
		case Fixed:
			k := 0
			if !last.Before(t.StartOn) {
				k = last.Sub(t.StartOn)/t.IntervalDays + 1
			}
			due = t.StartOn.AddDays(k * t.IntervalDays)
		default:
			due = last.AddDays(t.interval(last, h))
		}
	}
	// A snooze counts only while it is the latest event: a later done or skip clears it.
	// Sorting is stable, so a snooze logged after a done on the same day is still latest.
	if n := len(sorted); n > 0 && sorted[n-1].Kind == Snoozed && sorted[n-1].SnoozeUntil.After(due) {
		due = sorted[n-1].SnoozeUntil
	}
	return due
}

// lastCompletion is the index of the latest done or skipped event, or -1.
func lastCompletion(sorted []Event) int {
	for i := len(sorted) - 1; i >= 0; i-- {
		if sorted[i].Kind != Snoozed {
			return i
		}
	}
	return -1
}

// LastDone returns the latest date the task was done; skips and snoozes don't count.
func LastDone(events []Event) (Date, bool) {
	var last Date
	found := false
	for _, e := range events {
		if e.Kind == Done && (!found || e.On.After(last)) {
			last, found = e.On, true
		}
	}
	return last, found
}
