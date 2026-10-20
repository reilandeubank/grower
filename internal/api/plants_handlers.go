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
	unknownSpecies  = "That species does not exist."
)

// ListPlants returns active plants, or only archived ones when asked.
func (h *Handler) ListPlants(ctx context.Context, req apigen.ListPlantsRequestObject) (apigen.ListPlantsResponseObject, error) {
	archived := req.Params.Archived != nil && *req.Params.Archived
	rows, err := h.db.Reader().ListPlants(ctx, archived)
	if err != nil {
		return nil, fmt.Errorf("list plants: %w", err)
	}
	covers, err := h.covers(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.Plant, len(rows))
	for i, r := range rows {
		if items[i], err = plant(r, covers[r.ID]); err != nil {
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
		ID: db.NewID(), Name: in.name, Species: in.species, SpeciesID: in.speciesID, LocationID: in.locationID, AcquiredOn: in.acquiredOn,
		Notes: in.notes, PotType: in.potType, PotSizeIn: in.potSizeIn, Drainage: in.drainage, Soil: in.soil,
		CreatedAt: now, UpdatedAt: now,
	})
	switch {
	case db.IsForeignKeyViolation(err):
		return apigen.CreatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(h.missingReference(ctx, in))}, nil
	case err != nil:
		return nil, fmt.Errorf("create plant: %w", err)
	}
	p, err := plant(row, nil)
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
	p, err := h.plantWithCover(ctx, row)
	if err != nil {
		return nil, err
	}
	return apigen.GetPlant200JSONResponse(p), nil
}

// UpdatePlant replaces a plant's fields and archives or restores it.
func (h *Handler) UpdatePlant(ctx context.Context, req apigen.UpdatePlantRequestObject) (apigen.UpdatePlantResponseObject, error) {
	b := req.Body
	in, detail := parsePlant(apigen.PlantInput{
		Name: b.Name, Species: b.Species, SpeciesId: b.SpeciesId, LocationId: b.LocationId, AcquiredOn: b.AcquiredOn, Notes: b.Notes,
		PotType: b.PotType, PotSizeIn: b.PotSizeIn, Drainage: b.Drainage, Soil: b.Soil,
	})
	if detail != "" {
		return apigen.UpdatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	row, err := h.db.Writer().UpdatePlant(ctx, dbgen.UpdatePlantParams{
		Name: in.name, Species: in.species, SpeciesID: in.speciesID, LocationID: in.locationID, AcquiredOn: in.acquiredOn, Notes: in.notes,
		PotType: in.potType, PotSizeIn: in.potSizeIn, Drainage: in.drainage, Soil: in.soil,
		UpdatedAt: db.FormatTime(h.auth.Now()), Archived: b.Archived, ID: req.Id,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UpdatePlant404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case db.IsForeignKeyViolation(err):
		return apigen.UpdatePlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(h.missingReference(ctx, in))}, nil
	case err != nil:
		return nil, fmt.Errorf("update plant %s: %w", req.Id, err)
	}
	p, err := h.plantWithCover(ctx, row)
	if err != nil {
		return nil, err
	}
	return apigen.UpdatePlant200JSONResponse(p), nil
}

// DeletePlant removes a plant permanently, with its photos and their files.
func (h *Handler) DeletePlant(ctx context.Context, req apigen.DeletePlantRequestObject) (apigen.DeletePlantResponseObject, error) {
	var photoIDs []string
	var n int64
	err := h.db.InTx(ctx, func(q *dbgen.Queries) error {
		var err error
		if photoIDs, err = q.ListPlantPhotoIDs(ctx, req.Id); err != nil {
			return err
		}
		n, err = q.DeletePlant(ctx, req.Id) // photos go by ON DELETE CASCADE
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("delete plant %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeletePlant404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	}
	h.removeFiles(ctx, photoIDs...)
	return apigen.DeletePlant204Response{}, nil
}

// missingReference names the reference behind a foreign key violation; SQLite doesn't
// say which key failed.
func (h *Handler) missingReference(ctx context.Context, in plantFields) string {
	if in.speciesID.Valid {
		if _, err := h.db.Reader().GetSpecies(ctx, in.speciesID.String); errors.Is(err, sql.ErrNoRows) {
			return unknownSpecies
		}
	}
	return unknownLocation
}

type plantFields struct {
	name, species, notes, soil string
	locationID, speciesID      sql.NullString
	acquiredOn, potType        sql.NullString
	potSizeIn                  sql.NullFloat64
	drainage                   sql.NullInt64
}

// parsePlant validates input, returning a user-facing detail when it is invalid.
// Whether location_id and species_id exist is left to the foreign keys.
func parsePlant(in apigen.PlantInput) (plantFields, string) {
	f := plantFields{
		name: strings.TrimSpace(in.Name), species: deref(in.Species), notes: deref(in.Notes),
		soil: strings.TrimSpace(deref(in.Soil)),
	}
	switch {
	case f.name == "":
		return f, "Name is required."
	case utf8.RuneCountInString(f.name) > 80:
		return f, "Name must be 80 characters or fewer."
	case utf8.RuneCountInString(f.species) > 120:
		return f, "Species must be 120 characters or fewer."
	case utf8.RuneCountInString(f.notes) > 4000:
		return f, "Notes must be 4000 characters or fewer."
	case in.PotType != nil && !in.PotType.Valid():
		return f, "Pot type must be plastic, terracotta, ceramic, fabric, self_watering or other."
	case in.PotSizeIn != nil && (*in.PotSizeIn < 0.5 || *in.PotSizeIn > 48):
		return f, "Pot size must be 0.5 to 48 inches."
	case utf8.RuneCountInString(f.soil) > 120:
		return f, "Soil must be 120 characters or fewer."
	}
	if in.SpeciesId != nil {
		f.speciesID = sql.NullString{String: *in.SpeciesId, Valid: true}
	}
	if in.LocationId != nil {
		f.locationID = sql.NullString{String: *in.LocationId, Valid: true}
	}
	if in.AcquiredOn != nil {
		f.acquiredOn = sql.NullString{String: in.AcquiredOn.String(), Valid: true}
	}
	if in.PotType != nil {
		f.potType = sql.NullString{String: string(*in.PotType), Valid: true}
	}
	if in.PotSizeIn != nil {
		f.potSizeIn = sql.NullFloat64{Float64: *in.PotSizeIn, Valid: true}
	}
	if in.Drainage != nil {
		f.drainage = sql.NullInt64{Int64: boolInt(*in.Drainage), Valid: true}
	}
	return f, ""
}

// plant maps a row and its cover photo, which is nil when it has none.
func plant(r dbgen.Plant, cover *dbgen.Photo) (apigen.Plant, error) {
	created, err := db.ParseTime(r.CreatedAt)
	if err != nil {
		return apigen.Plant{}, err
	}
	p := apigen.Plant{
		Id: r.ID, Name: r.Name, Species: r.Species, Notes: r.Notes,
		Archived: r.ArchivedAt.Valid, CreatedAt: created, Soil: r.Soil,
	}
	if r.PotType.Valid {
		t := apigen.PotType(r.PotType.String)
		p.PotType = &t
	}
	if r.PotSizeIn.Valid {
		p.PotSizeIn = &r.PotSizeIn.Float64
	}
	if r.Drainage.Valid {
		d := r.Drainage.Int64 == 1
		p.Drainage = &d
	}
	if r.LocationID.Valid {
		p.LocationId = &r.LocationID.String
	}
	if r.SpeciesID.Valid {
		p.SpeciesId = &r.SpeciesID.String
	}
	if r.AcquiredOn.Valid {
		d, err := time.Parse(time.DateOnly, r.AcquiredOn.String)
		if err != nil {
			return apigen.Plant{}, fmt.Errorf("parse stored date %q: %w", r.AcquiredOn.String, err)
		}
		p.AcquiredOn = &openapi_types.Date{Time: d}
	}
	if cover != nil {
		c, err := photo(*cover)
		if err != nil {
			return apigen.Plant{}, err
		}
		p.Cover = &c
	}
	return p, nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
