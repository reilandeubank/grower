package api

import (
	"context"
	"fmt"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/push"
)

// GetHomeAssistantSummary is Today shaped for a Home Assistant REST sensor: counts for
// the state, and the reminder's own wording for notifications and announcements.
func (h *Handler) GetHomeAssistantSummary(ctx context.Context, _ apigen.GetHomeAssistantSummaryRequestObject) (apigen.GetHomeAssistantSummaryResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	agenda, err := care.Today(ctx, h.db.Reader(), cal)
	if err != nil {
		return nil, err
	}
	locations, err := h.db.Reader().ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	rooms := make(map[string]string, len(locations))
	for _, l := range locations {
		rooms[l.ID] = l.Name
	}
	return apigen.GetHomeAssistantSummary200JSONResponse(homeAssistantSummary(agenda, rooms)), nil
}

func homeAssistantSummary(a care.Agenda, rooms map[string]string) apigen.HomeAssistantSummary {
	phrases := push.Phrases(a, rooms)
	res := apigen.HomeAssistantSummary{
		Date:      apiDate(a.Date),
		NeedsCare: len(phrases),
		Overdue:   len(a.Overdue),
		Due:       len(a.Due),
		Upcoming:  len(a.Upcoming),
		Items:     make([]apigen.HomeAssistantItem, len(phrases)),
	}
	if m, ok := push.DigestMessage(a, rooms); ok {
		res.Title, res.Message = m.Title, m.Body
	}
	plants := map[string]bool{}
	for i, ph := range phrases {
		it := ph.Item
		plants[it.Plant.ID] = true
		item := apigen.HomeAssistantItem{
			PlantId:     it.Plant.ID,
			Plant:       it.PlantName,
			TaskId:      it.Task.ID,
			Task:        it.Task.Kind,
			Label:       ph.Text,
			DaysOverdue: it.DaysOverdue,
		}
		if room, ok := rooms[it.Plant.LocationID.String]; ok && it.Plant.LocationID.Valid {
			item.Location = &room
		}
		res.Items[i] = item
	}
	res.Plants = len(plants)
	return res
}
