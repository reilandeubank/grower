// Package hass builds what grower tells Home Assistant: the care summary that the REST
// endpoint returns and the MQTT publisher sends, so the two always agree.
package hass

import (
	"context"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/push"
	"github.com/reilandeubank/grower/internal/schedule"
)

// Rooms maps location IDs to their names.
func Rooms(ctx context.Context, q *dbgen.Queries) (map[string]string, error) {
	locations, err := q.ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	rooms := make(map[string]string, len(locations))
	for _, l := range locations {
		rooms[l.ID] = l.Name
	}
	return rooms, nil
}

// Summary is what needs care in a, in the daily reminder's words. rooms (location ID to
// name) tells apart plants that share a name.
func Summary(a care.Agenda, rooms map[string]string) apigen.HomeAssistantSummary {
	phrases := push.Phrases(a, rooms)
	res := apigen.HomeAssistantSummary{
		Date:      Date(a.Date),
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

// Date is d as the API's date type.
func Date(d schedule.Date) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)}
}
