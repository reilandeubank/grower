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
)

const (
	duplicateLocation = "A location with that name already exists."
	locationNotFound  = "Location not found."
)

// ListLocations returns every location with its active plant count.
func (h *Handler) ListLocations(ctx context.Context, _ apigen.ListLocationsRequestObject) (apigen.ListLocationsResponseObject, error) {
	rows, err := h.db.Reader().ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	items := make([]apigen.Location, len(rows))
	for i, r := range rows {
		items[i] = location(dbgen.GetLocationRow(r))
	}
	return apigen.ListLocations200JSONResponse{Items: items}, nil
}

// CreateLocation adds a location; names are unique ignoring case.
func (h *Handler) CreateLocation(ctx context.Context, req apigen.CreateLocationRequestObject) (apigen.CreateLocationResponseObject, error) {
	in, detail := parseLocation(*req.Body)
	if detail != "" {
		return apigen.CreateLocation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	now := db.FormatTime(h.auth.Now())
	row := dbgen.GetLocationRow{ID: db.NewID(), Name: in.name, Light: in.light, Notes: in.notes}
	err := h.db.Writer().CreateLocation(ctx, dbgen.CreateLocationParams{
		ID: row.ID, Name: row.Name, Light: row.Light, Notes: row.Notes, CreatedAt: now, UpdatedAt: now,
	})
	switch {
	case db.IsUniqueViolation(err):
		return apigen.CreateLocation409ApplicationProblemPlusJSONResponse(problem(http.StatusConflict, duplicateLocation)), nil
	case err != nil:
		return nil, fmt.Errorf("create location: %w", err)
	}
	return apigen.CreateLocation201JSONResponse(location(row)), nil
}

// UpdateLocation replaces a location's fields.
func (h *Handler) UpdateLocation(ctx context.Context, req apigen.UpdateLocationRequestObject) (apigen.UpdateLocationResponseObject, error) {
	in, detail := parseLocation(*req.Body)
	if detail != "" {
		return apigen.UpdateLocation400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	n, err := h.db.Writer().UpdateLocation(ctx, dbgen.UpdateLocationParams{
		Name: in.name, Light: in.light, Notes: in.notes, UpdatedAt: db.FormatTime(h.auth.Now()), ID: req.Id,
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
	return apigen.UpdateLocation200JSONResponse(location(row)), nil
}

// DeleteLocation removes a location; the schema unplaces its plants (ON DELETE SET NULL).
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

type locationFields struct {
	name  string
	light sql.NullString
	notes string
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
	return f, ""
}

func location(r dbgen.GetLocationRow) apigen.Location {
	l := apigen.Location{Id: r.ID, Name: r.Name, Notes: r.Notes, PlantCount: int(r.PlantCount)}
	if r.Light.Valid {
		light := apigen.LightLevel(r.Light.String)
		l.Light = &light
	}
	return l
}
