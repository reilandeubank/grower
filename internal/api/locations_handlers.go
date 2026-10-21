package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/light"
	"github.com/reilandeubank/grower/internal/schedule"
)

const (
	duplicateLocation = "A location with that name already exists."
	locationNotFound  = "Location not found."
)

// ListLocations returns every location with its windows and active plant count.
func (h *Handler) ListLocations(ctx context.Context, _ apigen.ListLocationsRequestObject) (apigen.ListLocationsResponseObject, error) {
	hs, err := h.hemisphere(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := h.db.Reader().ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	windows, err := allWindows(ctx, h.db.Reader())
	if err != nil {
		return nil, err
	}
	items := make([]apigen.Location, len(rows))
	for i, r := range rows {
		items[i] = location(dbgen.GetLocationRow(r), windows[r.ID], hs)
	}
	return apigen.ListLocations200JSONResponse{Items: items}, nil
}

// CreateLocation adds a location with its windows; names are unique ignoring case.
func (h *Handler) CreateLocation(ctx context.Context, req apigen.CreateLocationRequestObject) (apigen.CreateLocationResponseObject, error) {
	in, detail := parseLocation(*req.Body)
	if detail != "" {
		return apigen.CreateLocation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	hs, err := h.hemisphere(ctx)
	if err != nil {
		return nil, err
	}
	now := db.FormatTime(h.auth.Now())
	row := dbgen.GetLocationRow{ID: db.NewID(), Name: in.name, Light: in.light, Notes: in.notes}
	err = h.db.InTx(ctx, func(q *dbgen.Queries) error {
		err := q.CreateLocation(ctx, dbgen.CreateLocationParams{
			ID: row.ID, Name: row.Name, Light: row.Light, Notes: row.Notes, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return replaceWindows(ctx, q, row.ID, in.windows)
	})
	switch {
	case db.IsUniqueViolation(err):
		return apigen.CreateLocation409ApplicationProblemPlusJSONResponse(problem(http.StatusConflict, duplicateLocation)), nil
	case err != nil:
		return nil, fmt.Errorf("create location: %w", err)
	}
	return apigen.CreateLocation201JSONResponse(location(row, in.windows, hs)), nil
}

// UpdateLocation replaces a location's fields and its set of windows.
func (h *Handler) UpdateLocation(ctx context.Context, req apigen.UpdateLocationRequestObject) (apigen.UpdateLocationResponseObject, error) {
	in, detail := parseLocation(*req.Body)
	if detail != "" {
		return apigen.UpdateLocation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	hs, err := h.hemisphere(ctx)
	if err != nil {
		return nil, err
	}
	var n int64
	err = h.db.InTx(ctx, func(q *dbgen.Queries) error {
		var err error
		n, err = q.UpdateLocation(ctx, dbgen.UpdateLocationParams{
			Name: in.name, Light: in.light, Notes: in.notes, UpdatedAt: db.FormatTime(h.auth.Now()), ID: req.Id,
		})
		if err != nil || n == 0 {
			return err
		}
		return replaceWindows(ctx, q, req.Id, in.windows)
	})
	switch {
	case db.IsUniqueViolation(err):
		return apigen.UpdateLocation409ApplicationProblemPlusJSONResponse(problem(http.StatusConflict, duplicateLocation)), nil
	case err != nil:
		return nil, fmt.Errorf("update location %s: %w", req.Id, err)
	case n == 0:
		return apigen.UpdateLocation404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, locationNotFound)), nil
	}
	row, err := h.db.Reader().GetLocation(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("reload location %s: %w", req.Id, err)
	}
	return apigen.UpdateLocation200JSONResponse(location(row, in.windows, hs)), nil
}

// DeleteLocation removes a location; the schema unplaces its plants (ON DELETE SET NULL)
// and drops its windows (ON DELETE CASCADE).
func (h *Handler) DeleteLocation(ctx context.Context, req apigen.DeleteLocationRequestObject) (apigen.DeleteLocationResponseObject, error) {
	n, err := h.db.Writer().DeleteLocation(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete location %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeleteLocation404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, locationNotFound)), nil
	}
	return apigen.DeleteLocation204Response{}, nil
}

// replaceWindows swaps a location's windows for a new set, inside the caller's transaction.
func replaceWindows(ctx context.Context, q *dbgen.Queries, id string, windows []light.Window) error {
	if err := q.DeleteLocationWindows(ctx, id); err != nil {
		return err
	}
	for _, w := range windows {
		err := q.CreateLocationWindow(ctx, dbgen.CreateLocationWindowParams{LocationID: id, Direction: string(w.Direction), Count: int64(w.Count)})
		if err != nil {
			return err
		}
	}
	return nil
}

type locationFields struct {
	name    string
	light   sql.NullString
	notes   string
	windows []light.Window
}

// parseLocation validates input, returning a user-facing detail when it is invalid.
func parseLocation(in apigen.LocationInput) (locationFields, string) {
	f := locationFields{name: strings.TrimSpace(in.Name), notes: deref(in.Notes)}
	switch {
	case f.name == "":
		return f, "Name is required."
	case utf8.RuneCountInString(f.name) > 60:
		return f, "Name must be 60 characters or fewer."
	case utf8.RuneCountInString(f.notes) > 2000:
		return f, "Notes must be 2000 characters or fewer."
	case in.Light != nil && !in.Light.Valid():
		return f, "Light must be low, medium, bright_indirect or direct."
	}
	if in.Light != nil {
		f.light = sql.NullString{String: string(*in.Light), Valid: true}
	}
	if in.Windows != nil {
		seen := make(map[apigen.Direction]bool)
		for _, w := range *in.Windows {
			switch {
			case !w.Direction.Valid():
				return f, "Window direction must be n, ne, e, se, s, sw, w or nw."
			case w.Count < 1 || w.Count > 9:
				return f, "Window count must be 1 to 9."
			case seen[w.Direction]:
				return f, "List each window direction once."
			}
			seen[w.Direction] = true
			f.windows = append(f.windows, light.Window{Direction: light.Direction(w.Direction), Count: w.Count})
		}
		light.SortWindows(f.windows)
	}
	return f, ""
}

// location maps a row with its windows, in compass order, and what they suggest in the
// user's hemisphere.
func location(r dbgen.GetLocationRow, windows []light.Window, hs schedule.Hemisphere) apigen.Location {
	l := apigen.Location{Id: r.ID, Name: r.Name, Notes: r.Notes, PlantCount: int(r.PlantCount), Windows: apiWindows(windows)}
	if r.Light.Valid {
		lv := apigen.LightLevel(r.Light.String)
		l.Light = &lv
	}
	if s, ok := light.Suggested(windows, hs); ok {
		lv := apigen.LightLevel(s)
		l.SuggestedLight = &lv
	}
	return l
}
