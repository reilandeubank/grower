// Package care loads care tasks and sorts them into the Today agenda. The API's Today
// screen and the daily reminder both read it, so they always agree on what is due.
package care

import (
	"context"
	"fmt"
	"slices"

	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

// UpcomingDays is how far ahead Today looks.
const UpcomingDays = 7

// Calendar is where the user is in the year: their date and hemisphere.
type Calendar struct {
	Today      schedule.Date
	Hemisphere schedule.Hemisphere
}

// Dates is a task's schedule: its start and what the events make of it.
type Dates struct {
	Start schedule.Date
	Next  schedule.Date
	// Interval is the interval this season (schedule.CurrentInterval).
	Interval int
	// Last is the most recent completion; zero when HasLast is false.
	Last    schedule.Date
	HasLast bool
	// DryingDays is the learned drying time of a moisture-check task (schedule.DryingDays);
	// zero when HasDrying is false.
	DryingDays int
	HasDrying  bool
	// LastReading is the latest event with a meter reading; Reading is 0 when there is none.
	LastReading schedule.Event
}

// Item is one task of an active plant on the agenda.
type Item struct {
	Task  dbgen.CareTask
	Plant dbgen.Plant
	Dates Dates
	// DaysOverdue is positive when late, 0 when due today and negative when upcoming.
	DaysOverdue int
}

// Agenda is what Today shows: overdue, due today, and the coming week.
type Agenda struct {
	Date schedule.Date
	// TaskCount counts every task of an active plant, listed or not.
	TaskCount int
	Overdue   []Item
	Due       []Item
	Upcoming  []Item
}

// Today sorts every task of an active plant into the agenda for cal.Today.
func Today(ctx context.Context, q *dbgen.Queries, cal Calendar) (Agenda, error) {
	date := cal.Today
	// Two queries in all: tasks with their plants, and every event they need.
	rows, err := q.ListActiveTasks(ctx)
	if err != nil {
		return Agenda{}, fmt.Errorf("list active tasks: %w", err)
	}
	all, err := q.ListActiveEvents(ctx)
	if err != nil {
		return Agenda{}, fmt.Errorf("list active events: %w", err)
	}
	events := make(map[string][]dbgen.CareEvent)
	for _, e := range all {
		events[e.TaskID] = append(events[e.TaskID], e)
	}

	a := Agenda{Date: date, TaskCount: len(rows), Overdue: []Item{}, Due: []Item{}, Upcoming: []Item{}}
	for _, r := range rows {
		dates, err := Schedule(r.CareTask, events[r.CareTask.ID], cal)
		if err != nil {
			return Agenda{}, err
		}
		item := Item{Task: r.CareTask, Plant: r.Plant, Dates: dates, DaysOverdue: date.Sub(dates.Next)}
		switch {
		case item.DaysOverdue > 0:
			a.Overdue = append(a.Overdue, item)
		case item.DaysOverdue == 0:
			a.Due = append(a.Due, item)
		case item.DaysOverdue >= -UpcomingDays:
			a.Upcoming = append(a.Upcoming, item)
		}
	}
	// Most overdue or soonest first. Rows come ordered by plant name, and the sort is
	// stable, so that breaks ties.
	byDays := func(a, b Item) int { return b.DaysOverdue - a.DaysOverdue }
	slices.SortStableFunc(a.Overdue, byDays)
	slices.SortStableFunc(a.Upcoming, byDays)
	return a, nil
}

// Schedule computes a task's next due date and last completion from its non-voided
// events in insertion order, in the seasons of cal.
func Schedule(row dbgen.CareTask, events []dbgen.CareEvent, cal Calendar) (Dates, error) {
	start, err := schedule.Parse(row.StartOn)
	if err != nil {
		return Dates{}, err
	}
	evs := make([]schedule.Event, len(events))
	for i, e := range events {
		evs[i] = schedule.Event{Kind: schedule.EventKind(e.Kind), Reading: int(e.Reading.Int64)}
		if evs[i].On, err = schedule.Parse(e.LocalDate); err != nil {
			return Dates{}, err
		}
		if e.SnoozeUntil.Valid {
			if evs[i].SnoozeUntil, err = schedule.Parse(e.SnoozeUntil.String); err != nil {
				return Dates{}, err
			}
		}
	}
	task := schedule.Task{
		IntervalDays: int(row.IntervalDays), Anchor: schedule.Anchor(row.Anchor), StartOn: start, Kind: row.Kind, Seasonal: row.Seasonal == 1,
		MoistureCheck: row.MoistureCheck == 1, WaterAt: int(row.WaterAt),
	}
	d := Dates{
		Start: start, Next: schedule.NextDue(task, evs, cal.Hemisphere), Interval: schedule.CurrentInterval(task, evs, cal.Today, cal.Hemisphere),
	}
	d.Last, d.HasLast = schedule.LastDone(evs)
	d.DryingDays, d.HasDrying = schedule.DryingDays(task, evs)
	d.LastReading, _ = schedule.LastReading(evs)
	return d, nil
}
