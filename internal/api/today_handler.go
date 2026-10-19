package api

import (
	"context"
	"fmt"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

// GetToday sorts every task of an active plant into overdue, due today or upcoming.
func (h *Handler) GetToday(ctx context.Context, _ apigen.GetTodayRequestObject) (apigen.GetTodayResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	agenda, err := care.Today(ctx, h.db.Reader(), cal)
	if err != nil {
		return nil, err
	}
	covers, err := h.covers(ctx)
	if err != nil {
		return nil, err
	}
	res := apigen.Today{Date: apiDate(cal.Today), TaskCount: agenda.TaskCount}
	if res.Overdue, err = todayItems(agenda.Overdue, covers); err != nil {
		return nil, err
	}
	if res.Due, err = todayItems(agenda.Due, covers); err != nil {
		return nil, err
	}
	if res.Upcoming, err = todayItems(agenda.Upcoming, covers); err != nil {
		return nil, err
	}
	return apigen.GetToday200JSONResponse(res), nil
}

func todayItems(items []care.Item, covers map[string]*dbgen.Photo) ([]apigen.TodayItem, error) {
	out := make([]apigen.TodayItem, len(items))
	for i, it := range items {
		p, err := plant(it.Plant, covers[it.Plant.ID])
		if err != nil {
			return nil, fmt.Errorf("map plant %s: %w", it.Plant.ID, err)
		}
		out[i] = apigen.TodayItem{Task: taskOf(it.Task, it.Dates), Plant: p, DaysOverdue: it.DaysOverdue}
	}
	return out, nil
}
