package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

const (
	speciesNotFound   = "Species not found."
	duplicateSpecies  = "That species is already saved."
	speciesNameLength = 120
)

// ListSpecies returns saved species, optionally matching q in either name. It never
// calls a provider.
func (h *Handler) ListSpecies(ctx context.Context, req apigen.ListSpeciesRequestObject) (apigen.ListSpeciesResponseObject, error) {
	rows, err := h.db.Reader().ListSpecies(ctx, strings.TrimSpace(deref(req.Params.Q)))
	if err != nil {
		return nil, fmt.Errorf("list species: %w", err)
	}
	items := make([]apigen.Species, len(rows))
	for i, r := range rows {
		if items[i], err = speciesOf(dbgen.GetSpeciesRow(r)); err != nil {
			return nil, err
		}
	}
	return apigen.ListSpecies200JSONResponse{Items: items}, nil
}

// CreateSpecies saves a species by hand. Its values are the person's own, so it counts
// as user_edited and a later import with the same name won't overwrite them.
func (h *Handler) CreateSpecies(ctx context.Context, req apigen.CreateSpeciesRequestObject) (apigen.CreateSpeciesResponseObject, error) {
	in, detail := parseSpecies(*req.Body)
	if detail != "" {
		return apigen.CreateSpecies400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	now := db.FormatTime(h.auth.Now())
	in.ID, in.Source, in.Raw, in.UserEdited, in.CreatedAt, in.UpdatedAt = db.NewID(), "manual", "{}", 1, now, now
	err := h.db.Writer().CreateSpecies(ctx, in)
	switch {
	case db.IsUniqueViolation(err):
		return apigen.CreateSpecies409ApplicationProblemPlusJSONResponse(problem(http.StatusConflict, duplicateSpecies)), nil
	case err != nil:
		return nil, fmt.Errorf("create species: %w", err)
	}
	s, err := h.species(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return apigen.CreateSpecies201JSONResponse(s), nil
}

// GetSpecies returns one saved species.
func (h *Handler) GetSpecies(ctx context.Context, req apigen.GetSpeciesRequestObject) (apigen.GetSpeciesResponseObject, error) {
	s, err := h.species(ctx, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return apigen.GetSpecies404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, speciesNotFound)), nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.GetSpecies200JSONResponse(s), nil
}

// UpdateSpecies replaces a species' names and care defaults and marks it user_edited.
func (h *Handler) UpdateSpecies(ctx context.Context, req apigen.UpdateSpeciesRequestObject) (apigen.UpdateSpeciesResponseObject, error) {
	in, detail := parseSpecies(*req.Body)
	if detail != "" {
		return apigen.UpdateSpecies400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	n, err := h.db.Writer().UpdateSpecies(ctx, dbgen.UpdateSpeciesParams{
		ScientificName: in.ScientificName, CommonName: in.CommonName, WaterIntervalDays: in.WaterIntervalDays,
		Light: in.Light, WaterAt: in.WaterAt, Notes: in.Notes, UpdatedAt: db.FormatTime(h.auth.Now()), ID: req.Id,
	})
	switch {
	case db.IsUniqueViolation(err):
		return apigen.UpdateSpecies409ApplicationProblemPlusJSONResponse(problem(http.StatusConflict, duplicateSpecies)), nil
	case err != nil:
		return nil, fmt.Errorf("update species %s: %w", req.Id, err)
	case n == 0:
		return apigen.UpdateSpecies404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, speciesNotFound)), nil
	}
	s, err := h.species(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateSpecies200JSONResponse(s), nil
}

// species loads one species for a response; sql.ErrNoRows passes through unwrapped.
func (h *Handler) species(ctx context.Context, id string) (apigen.Species, error) {
	row, err := h.db.Reader().GetSpecies(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return apigen.Species{}, err
	}
	if err != nil {
		return apigen.Species{}, fmt.Errorf("get species %s: %w", id, err)
	}
	return speciesOf(row)
}

// parseSpecies validates input, returning a user-facing detail when it is invalid.
func parseSpecies(in apigen.SpeciesInput) (dbgen.CreateSpeciesParams, string) {
	f := dbgen.CreateSpeciesParams{
		ScientificName: strings.TrimSpace(in.ScientificName), CommonName: strings.TrimSpace(deref(in.CommonName)), Notes: deref(in.Notes),
	}
	switch {
	case f.ScientificName == "":
		return f, "Scientific name is required."
	case utf8.RuneCountInString(f.ScientificName) > speciesNameLength:
		return f, "Scientific name must be 120 characters or fewer."
	case utf8.RuneCountInString(f.CommonName) > speciesNameLength:
		return f, "Common name must be 120 characters or fewer."
	case in.WaterIntervalDays != nil && (*in.WaterIntervalDays < 1 || *in.WaterIntervalDays > 365):
		return f, "Water interval must be 1 to 365 days."
	case in.Light != nil && !in.Light.Valid():
		return f, "Light must be low, medium, bright_indirect or direct."
	case in.WaterAt != nil && (*in.WaterAt < 1 || *in.WaterAt > 9):
		return f, "Water-at must be between 1 and 9."
	case utf8.RuneCountInString(f.Notes) > 2000:
		return f, "Notes must be 2000 characters or fewer."
	}
	f.WaterIntervalDays, f.WaterAt = nullInt(in.WaterIntervalDays), nullInt(in.WaterAt)
	if in.Light != nil {
		f.Light = sql.NullString{String: string(*in.Light), Valid: true}
	}
	return f, ""
}

func speciesOf(r dbgen.GetSpeciesRow) (apigen.Species, error) {
	s := apigen.Species{
		Id: r.ID, ScientificName: r.ScientificName, CommonName: r.CommonName, Source: apigen.SpeciesSource(r.Source),
		SourceRef: r.SourceRef, Notes: r.Notes, UserEdited: r.UserEdited == 1, PlantCount: int(r.PlantCount),
		WaterIntervalDays: intPtr(r.WaterIntervalDays), WaterAt: intPtr(r.WaterAt),
	}
	if r.Light.Valid {
		l := apigen.LightLevel(r.Light.String)
		s.Light = &l
	}
	var err error
	s.FetchedAt, err = nullTime(r.FetchedAt)
	return s, err
}

func nullInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}
