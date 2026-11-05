// Package care loads care tasks and sorts them into the Today agenda. The API's Today
// screen and the daily reminder both read it, so they always agree on what is due.
package care

import (
	"cmp"
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
	// PlantName is what to call the plant (Names.Name).
	PlantName string
	Dates     Dates
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
	// Three queries in all: tasks with their plants, the events their schedules depend on,
	// and the species labels that name unnamed plants. (Rarely a fourth per soil-check task:
	// see loadEvents.)
	rows, err := q.ListActiveTasks(ctx)
	if err != nil {
		return Agenda{}, fmt.Errorf("list active tasks: %w", err)
	}
	names, err := LoadNames(ctx, q)
	if err != nil {
		return Agenda{}, err
	}
	// The query orders each plant's tasks; re-sort by the name shown, which for an
	// unnamed plant isn't its name column, keeping plants that share a name apart.
	slices.SortStableFunc(rows, func(a, b dbgen.ListActiveTasksRow) int {
		return cmp.Or(CompareNames(names.Name(a.Plant), names.Name(b.Plant)), cmp.Compare(a.Plant.ID, b.Plant.ID))
	})
	events, err := loadEvents(ctx, q, rows)
	if err != nil {
		return Agenda{}, err
	}

	a := Agenda{Date: date, TaskCount: len(rows), Overdue: []Item{}, Due: []Item{}, Upcoming: []Item{}}
	for _, r := range rows {
		dates, err := Schedule(r.CareTask, events[r.CareTask.ID], cal)
		if err != nil {
			return Agenda{}, err
		}
		item := Item{Task: r.CareTask, Plant: r.Plant, PlantName: names.Name(r.Plant), Dates: dates, DaysOverdue: date.Sub(dates.Next)}
		switch {
		case item.DaysOverdue > 0:
			a.Overdue = append(a.Overdue, item)
		case item.DaysOverdue == 0:
			a.Due = append(a.Due, item)
		case item.DaysOverdue >= -UpcomingDays:
			a.Upcoming = append(a.Upcoming, item)
		}
	}
	// Most overdue or soonest first. Rows were sorted by plant name, and the sort is
	// stable, so that breaks ties.
	byDays := func(a, b Item) int { return b.DaysOverdue - a.DaysOverdue }
	slices.SortStableFunc(a.Overdue, byDays)
	slices.SortStableFunc(a.Upcoming, byDays)
	return a, nil
}

// loadEvents returns the events each task's schedule depends on (ListScheduleEvents),
// by task. A soil-check task whose loaded events don't hold its recent drying cycles, as
// when it was watered twice on a day, gets its whole log instead.
func loadEvents(ctx context.Context, q *dbgen.Queries, rows []dbgen.ListActiveTasksRow) (map[string][]dbgen.CareEvent, error) {
	all, err := q.ListScheduleEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedule events: %w", err)
	}
	events := make(map[string][]dbgen.CareEvent)
	for _, e := range all {
		events[e.TaskID] = append(events[e.TaskID], e)
	}
	for _, r := range rows {
		if r.CareTask.MoistureCheck != 1 {
			continue
		}
		task, evs, err := scheduleInput(r.CareTask, events[r.CareTask.ID])
		if err != nil {
			return nil, err
		}
		if schedule.HasRecentCycles(task, evs) {
			continue
		}
		if events[r.CareTask.ID], err = q.ListTaskEvents(ctx, r.CareTask.ID); err != nil {
			return nil, fmt.Errorf("list task events: %w", err)
		}
	}
	return events, nil
}

// Schedule computes a task's next due date and last completion from its non-voided
// events in insertion order, in the seasons of cal.
func Schedule(row dbgen.CareTask, events []dbgen.CareEvent, cal Calendar) (Dates, error) {
	task, evs, err := scheduleInput(row, events)
	if err != nil {
		return Dates{}, err
	}
	d := Dates{
		Start: task.StartOn, Next: schedule.NextDue(task, evs, cal.Hemisphere), Interval: schedule.CurrentInterval(task, evs, cal.Today, cal.Hemisphere),
	}
	d.Last, d.HasLast = schedule.LastDone(evs)
	d.DryingDays, d.HasDrying = schedule.DryingDays(task, evs)
	d.LastReading, _ = schedule.LastReading(evs)
	return d, nil
}

// scheduleInput converts a task and its events for the schedule package.
func scheduleInput(row dbgen.CareTask, events []dbgen.CareEvent) (schedule.Task, []schedule.Event, error) {
	start, err := schedule.Parse(row.StartOn)
	if err != nil {
		return schedule.Task{}, nil, err
	}
	evs := make([]schedule.Event, len(events))
	for i, e := range events {
		evs[i] = schedule.Event{Kind: schedule.EventKind(e.Kind), Reading: int(e.Reading.Int64)}
		if evs[i].On, err = schedule.Parse(e.LocalDate); err != nil {
			return schedule.Task{}, nil, err
		}
		if e.SnoozeUntil.Valid {
			if evs[i].SnoozeUntil, err = schedule.Parse(e.SnoozeUntil.String); err != nil {
				return schedule.Task{}, nil, err
			}
		}
	}
	task := schedule.Task{
		IntervalDays: int(row.IntervalDays), Anchor: schedule.Anchor(row.Anchor), StartOn: start, Kind: row.Kind, Seasonal: row.Seasonal == 1,
		MoistureCheck: row.MoistureCheck == 1, WaterAt: int(row.WaterAt),
	}
	return task, evs, nil
}
