package api

import (
	"context"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/hass"
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
	rooms, err := hass.Rooms(ctx, h.db.Reader())
	if err != nil {
		return nil, err
	}
	return apigen.GetHomeAssistantSummary200JSONResponse(hass.Summary(agenda, rooms)), nil
}
