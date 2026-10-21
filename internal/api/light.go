package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/light"
	"github.com/reilandeubank/grower/internal/schedule"
)

// settings loads the signed-in user's settings.
func (h *Handler) settings(ctx context.Context) (dbgen.Setting, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dbgen.Setting{}, errors.New("no signed-in user")
	}
	s, err := h.db.Reader().GetSettings(ctx, p.UserID)
	if err != nil {
		return dbgen.Setting{}, fmt.Errorf("load settings for user %s: %w", p.UserID, err)
	}
	return s, nil
}

// hemisphere is the signed-in user's hemisphere, which decides what windows suggest.
func (h *Handler) hemisphere(ctx context.Context) (schedule.Hemisphere, error) {
	s, err := h.settings(ctx)
	return schedule.Hemisphere(s.Hemisphere), err
}

// allWindows returns every location's windows by location id, in one query.
func allWindows(ctx context.Context, q *dbgen.Queries) (map[string][]light.Window, error) {
	rows, err := q.ListWindows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list windows: %w", err)
	}
	return groupWindows(rows), nil
}

// groupWindows groups window rows by location, each in compass order.
func groupWindows(rows []dbgen.LocationWindow) map[string][]light.Window {
	m := make(map[string][]light.Window)
	for _, r := range rows {
		m[r.LocationID] = append(m[r.LocationID], light.Window{Direction: light.Direction(r.Direction), Count: int(r.Count)})
	}
	for _, ws := range m {
		light.SortWindows(ws)
	}
	return m
}

// rooms returns every location's light by id (absent when unknown), for plants'
// effective light. A fixed three queries, however many plants are mapped.
func (h *Handler) rooms(ctx context.Context) (map[string]light.Light, error) {
	hs, err := h.hemisphere(ctx)
	if err != nil {
		return nil, err
	}
	locs, err := h.db.Reader().ListLocationLights(ctx)
	if err != nil {
		return nil, fmt.Errorf("list location lights: %w", err)
	}
	windows, err := allWindows(ctx, h.db.Reader())
	if err != nil {
		return nil, err
	}
	m := make(map[string]light.Light, len(locs))
	for _, l := range locs {
		if room, ok := light.Room(light.Level(l.Light.String), windows[l.ID], hs); ok {
			m[l.ID] = room
		}
	}
	return m, nil
}

// effectiveLight is a plant's effective light, or nil when nothing is known. rooms may
// be nil when the room's light doesn't matter.
func effectiveLight(r dbgen.Plant, rooms map[string]light.Light) *apigen.EffectiveLight {
	l, ok := light.Effective(light.Level(r.Light.String), rooms[r.LocationID.String],
		light.Distance(r.WindowDistance.String), int(r.GrowLightHours.Int64))
	if !ok {
		return nil
	}
	return &apigen.EffectiveLight{Level: apigen.LightLevel(l.Level), Source: apigen.EffectiveLightSource(l.Source)}
}

func apiWindows(ws []light.Window) []apigen.Window {
	out := make([]apigen.Window, len(ws))
	for i, w := range ws {
		out[i] = apigen.Window{Direction: apigen.Direction(w.Direction), Count: w.Count}
	}
	return out
}
