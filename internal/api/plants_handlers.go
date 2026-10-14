package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

const (
	plantNotFound   = "Plant not found."
	unknownLocation = "That location does not exist."
)

// ListPlants returns active plants, or only archived ones when asked.
func (h *Handler) ListPlants(ctx context.Context, req apigen.ListPlantsRequestObject) (apigen.ListPlantsResponseObject, error) {
	archived := req.Params.Archived != nil && *req.Params.Archived
	rows, err := h.db.Reader().ListPlants(ctx, archived)
	if err != nil {
		return nil, fmt.Errorf("list plants: %w", err)
	}
	items := make([]apigen.Plant, len(rows))
	for i, r := range rows {
		if items[i], err = plant(r); err != nil {
			return nil, err
		}
	}
	return apigen.ListPlants200JSONResponse{Items: items}, nil
}

// CreatePlant adds a plant.
func (h *Handler) CreatePlant(ctx context.Context, req apigen.CreatePlantRequestObject) (apigen.CreatePlantResponseObject, error) {
	in, detail := parsePlant(*req.Body)
	if detail != "" {
		return apigen.CreatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	now := db.FormatTime(h.auth.Now())
	row, err := h.db.Writer().CreatePlant(ctx, dbgen.CreatePlantParams{
		ID: db.NewID(), Name: in.name, Species: in.species, LocationID: in.locationID,
		AcquiredOn: in.acquiredOn, Notes: in.notes, CreatedAt: now, UpdatedAt: now,
	})
	switch {
	case db.IsForeignKeyViolation(err):
		return apigen.CreatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(unknownLocation)}, nil
	case err != nil:
		return nil, fmt.Errorf("create plant: %w", err)
	}
	p, err := plant(row)
	if err != nil {
		return nil, err
	}
	return apigen.CreatePlant201JSONResponse(p), nil
}

// GetPlant returns one plant, archived or not.
func (h *Handler) GetPlant(ctx context.Context, req apigen.GetPlantRequestObject) (apigen.GetPlantResponseObject, error) {
	row, err := h.db.Reader().GetPlant(ctx, req.Id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.GetPlant404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	p, err := plant(row)
	if err != nil {
		return nil, err
	}
	return apigen.GetPlant200JSONResponse(p), nil
}

// UpdatePlant replaces a plant's fields and archives or restores it.
func (h *Handler) UpdatePlant(ctx context.Context, req apigen.UpdatePlantRequestObject) (apigen.UpdatePlantResponseObject, error) {
	b := req.Body
	in, detail := parsePlant(apigen.PlantInput{
		Name: b.Name, Species: b.Species, LocationId: b.LocationId, AcquiredOn: b.AcquiredOn, Notes: b.Notes,
	})
	if detail != "" {
		return apigen.UpdatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	row, err := h.db.Writer().UpdatePlant(ctx, dbgen.UpdatePlantParams{
		Name: in.name, Species: in.species, LocationID: in.locationID, AcquiredOn: in.acquiredOn, Notes: in.notes,
		UpdatedAt: db.FormatTime(h.auth.Now()), Archived: b.Archived, ID: req.Id,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UpdatePlant404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case db.IsForeignKeyViolation(err):
		return apigen.UpdatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(unknownLocation)}, nil
	case err != nil:
		return nil, fmt.Errorf("update plant %s: %w", req.Id, err)
	}
	p, err := plant(row)
	if err != nil {
		return nil, err
	}
	return apigen.UpdatePlant200JSONResponse(p), nil
}

// DeletePlant removes a plant permanently.
func (h *Handler) DeletePlant(ctx context.Context, req apigen.DeletePlantRequestObject) (apigen.DeletePlantResponseObject, error) {
	n, err := h.db.Writer().DeletePlant(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete plant %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeletePlant404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	}
	return apigen.DeletePlant204Response{}, nil
}

type plantFields struct {
	name, species, notes   string
	locationID, acquiredOn sql.NullString
}

// parsePlant validates input, returning a user-facing detail when it is invalid.
// Whether location_id exists is left to the foreign key.
func parsePlant(in apigen.PlantInput) (plantFields, string) {
	f := plantFields{name: strings.TrimSpace(in.Name), species: deref(in.Species), notes: deref(in.Notes)}
	switch {
	case f.name == "":
		return f, "Name is required."
	case utf8.RuneCountInString(f.name) > 80:
		return f, "Name must be 80 characters or fewer."
	case utf8.RuneCountInString(f.species) > 120:
		return f, "Species must be 120 characters or fewer."
	case utf8.RuneCountInString(f.notes) > 4000:
		return f, "Notes must be 4000 characters or fewer."
	}
	if in.LocationId != nil {
		f.locationID = sql.NullString{String: *in.LocationId, Valid: true}
	}
	if in.AcquiredOn != nil {
		f.acquiredOn = sql.NullString{String: in.AcquiredOn.String(), Valid: true}
	}
	return f, ""
}

func plant(r dbgen.Plant) (apigen.Plant, error) {
	created, err := db.ParseTime(r.CreatedAt)
	if err != nil {
		return apigen.Plant{}, err
	}
	p := apigen.Plant{
		Id: r.ID, Name: r.Name, Species: r.Species, Notes: r.Notes,
		Archived: r.ArchivedAt.Valid, CreatedAt: created,
	}
	if r.LocationID.Valid {
		p.LocationId = &r.LocationID.String
	}
	if r.AcquiredOn.Valid {
		d, err := time.Parse(time.DateOnly, r.AcquiredOn.String)
		if err != nil {
			return apigen.Plant{}, fmt.Errorf("parse stored date %q: %w", r.AcquiredOn.String, err)
		}
		p.AcquiredOn = &openapi_types.Date{Time: d}
	}
	return p, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
