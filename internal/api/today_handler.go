package api

import (
	"context"
	"fmt"
	"slices"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

// upcomingDays is how far ahead Today looks.
const upcomingDays = 7

// GetToday sorts every task of an active plant into overdue, due today or upcoming.
func (h *Handler) GetToday(ctx context.Context, _ apigen.GetTodayRequestObject) (apigen.GetTodayResponseObject, error) {
	today, err := h.today(ctx)
	if err != nil {
		return nil, err
	}
	// Three queries in all: tasks with their plants, every event they need, and covers.
	q := h.db.Reader()
	rows, err := q.ListActiveTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	all, err := q.ListActiveEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active events: %w", err)
	}
	covers, err := h.covers(ctx)
	if err != nil {
		return nil, err
	}
	events := make(map[string][]dbgen.CareEvent)
	for _, e := range all {
		events[e.TaskID] = append(events[e.TaskID], e)
	}

	res := apigen.Today{Date: apiDate(today), TaskCount: len(rows), Overdue: []apigen.TodayItem{}, Due: []apigen.TodayItem{}, Upcoming: []apigen.TodayItem{}}
	for _, r := range rows {
		task, err := careTask(r.CareTask, events[r.CareTask.ID])
		if err != nil {
			return nil, err
		}
		item := apigen.TodayItem{Task: task, DaysOverdue: today.Sub(schedule.DateOf(task.NextDue.Time))}
		if item.Plant, err = plant(r.Plant, covers[r.Plant.ID]); err != nil {
			return nil, err
		}
		switch {
		case item.DaysOverdue > 0:
			res.Overdue = append(res.Overdue, item)
		case item.DaysOverdue == 0:
			res.Due = append(res.Due, item)
		case item.DaysOverdue >= -upcomingDays:
			res.Upcoming = append(res.Upcoming, item)
		}
	}
	// Most overdue or soonest first. Rows come ordered by plant name, and the sort is
	// stable, so that breaks ties.
	byDays := func(a, b apigen.TodayItem) int { return b.DaysOverdue - a.DaysOverdue }
	slices.SortStableFunc(res.Overdue, byDays)
	slices.SortStableFunc(res.Upcoming, byDays)
	return apigen.GetToday200JSONResponse(res), nil
}
