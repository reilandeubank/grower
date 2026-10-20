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
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/species"
)

// lookupProvider is the element type of SpeciesLookup.providers (generated inline).
type lookupProvider = struct {
	Detail *string                             `json:"detail,omitempty"`
	Ok     bool                                `json:"ok"`
	Source apigen.SpeciesLookupProvidersSource `json:"source"`
}

// LookupSpecies searches every configured provider at once and merges their
// candidates, one per scientific name and source, marking those already saved.
func (h *Handler) LookupSpecies(ctx context.Context, req apigen.LookupSpeciesRequestObject) (apigen.LookupSpeciesResponseObject, error) {
	q := strings.TrimSpace(req.Params.Q)
	if n := utf8.RuneCountInString(q); n < 2 || n > 100 {
		return apigen.LookupSpecies400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Search for 2 to 100 characters.")}, nil
	}
	names, err := h.db.Reader().ListSpeciesNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list species names: %w", err)
	}
	saved := make(map[string]string, len(names))
	for _, n := range names {
		saved[strings.ToLower(n.ScientificName)] = n.ID
	}

	out := apigen.SpeciesLookup{Providers: []lookupProvider{}, Items: []apigen.SpeciesCandidate{}}
	seen := map[string]bool{}
	for _, r := range h.lookup.Search(ctx, q) {
		status := lookupProvider{Source: apigen.SpeciesLookupProvidersSource(r.Provider.Source()), Ok: r.Err == nil}
		if r.Err != nil {
			detail := h.providerFailure(ctx, r.Provider, r.Err)
			status.Detail = &detail
		}
		out.Providers = append(out.Providers, status)
		if r.Err != nil {
			continue
		}
		for _, c := range r.Candidates {
			name := strings.ToLower(c.ScientificName)
			key := c.Source + "\x00" + name
			if seen[key] {
				continue
			}
			seen[key] = true
			item := apigen.SpeciesCandidate{
				Source: apigen.SpeciesCandidateSource(c.Source), Ref: c.Ref, ScientificName: c.ScientificName, CommonName: c.CommonName,
			}
			if id, ok := saved[name]; ok {
				item.SavedId = &id
			}
			out.Items = append(out.Items, item)
		}
	}
	return apigen.LookupSpecies200JSONResponse(out), nil
}

// ImportSpecies fetches a candidate's care details and saves them, matching saved
// species by scientific name.
func (h *Handler) ImportSpecies(ctx context.Context, req apigen.ImportSpeciesRequestObject) (apigen.ImportSpeciesResponseObject, error) {
	ref := strings.TrimSpace(req.Body.Ref)
	bad := func(detail string) (apigen.ImportSpeciesResponseObject, error) {
		return apigen.ImportSpecies400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	switch {
	case !req.Body.Source.Valid():
		return bad("Source must be openplantbook or perenual.")
	case ref == "" || utf8.RuneCountInString(ref) > 200:
		return bad("Ref must be 1 to 200 characters.")
	}
	p := h.lookup.Provider(string(req.Body.Source))
	if p == nil {
		return bad("That provider isn't set up on this server.")
	}
	c, err := p.Details(ctx, ref)
	if err == nil && strings.TrimSpace(c.ScientificName) == "" {
		err = fmt.Errorf("%s species %q has no scientific name", p.Name(), ref)
	}
	if err != nil {
		return apigen.ImportSpecies502ApplicationProblemPlusJSONResponse(problem(http.StatusBadGateway, h.providerFailure(ctx, p, err))), nil
	}
	id, err := h.saveImport(ctx, p.Source(), ref, c)
	if err != nil {
		return nil, err
	}
	s, err := h.species(ctx, id)
	if err != nil {
		return nil, err
	}
	return apigen.ImportSpecies200JSONResponse(s), nil
}

// saveImport inserts a new species, or refreshes the saved one: all provider values
// when nobody has edited it, otherwise only raw and fetched_at.
func (h *Handler) saveImport(ctx context.Context, source, ref string, c species.Care) (string, error) {
	now := db.FormatTime(h.auth.Now())
	fetched := sql.NullString{String: now, Valid: true}
	raw := string(c.Raw)
	if raw == "" {
		raw = "{}"
	}
	name, common := truncate(strings.TrimSpace(c.ScientificName)), truncate(strings.TrimSpace(c.CommonName))
	interval, waterAt := nullInt(c.WaterIntervalDays), nullInt(c.WaterAt)
	var light sql.NullString
	if c.Light != nil {
		light = sql.NullString{String: *c.Light, Valid: true}
	}
	var id string
	err := h.db.InTx(ctx, func(q *dbgen.Queries) error {
		saved, err := q.GetSpeciesByName(ctx, name)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			id = db.NewID()
			return q.CreateSpecies(ctx, dbgen.CreateSpeciesParams{
				ID: id, ScientificName: name, CommonName: common, Source: source, SourceRef: ref, WaterIntervalDays: interval,
				Light: light, WaterAt: waterAt, Raw: raw, FetchedAt: fetched, CreatedAt: now, UpdatedAt: now,
			})
		case err != nil:
			return err
		case saved.UserEdited == 1:
			id = saved.ID
			return q.RefreshSpeciesRaw(ctx, dbgen.RefreshSpeciesRawParams{Raw: raw, FetchedAt: fetched, UpdatedAt: now, ID: id})
		default:
			id = saved.ID
			return q.RefreshSpecies(ctx, dbgen.RefreshSpeciesParams{
				CommonName: common, Source: source, SourceRef: ref, WaterIntervalDays: interval, Light: light, WaterAt: waterAt,
				Raw: raw, FetchedAt: fetched, UpdatedAt: now, ID: id,
			})
		}
	})
	if err != nil {
		return "", fmt.Errorf("save imported species %s/%s: %w", source, ref, err)
	}
	return id, nil
}

// providerFailure is the user-facing reason a provider didn't answer. Real failures
// are logged here, since they become a response rather than a handler error.
func (h *Handler) providerFailure(ctx context.Context, p species.Provider, err error) string {
	if errors.Is(err, species.ErrDailyLimit) {
		return p.Name() + "'s daily limit is used up. Try again tomorrow."
	}
	h.log.WarnContext(ctx, "species provider failed", "provider", p.Source(), "err", err)
	return "Couldn't reach " + p.Name() + ". Try again later."
}

// ApplySpeciesCare puts the linked species' care defaults on the plant's water task.
func (h *Handler) ApplySpeciesCare(ctx context.Context, req apigen.ApplySpeciesCareRequestObject) (apigen.ApplySpeciesCareResponseObject, error) {
	bad := func(detail string) (apigen.ApplySpeciesCareResponseObject, error) {
		return apigen.ApplySpeciesCare400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	plant, err := h.db.Reader().GetPlant(ctx, req.Id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.ApplySpeciesCare404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	case !plant.SpeciesID.Valid:
		return bad("Link a species to this plant first.")
	}
	sp, err := h.db.Reader().GetSpecies(ctx, plant.SpeciesID.String)
	if err != nil {
		return nil, fmt.Errorf("get species %s: %w", plant.SpeciesID.String, err)
	}
	defaults := care.SpeciesDefaults{WaterIntervalDays: sp.WaterIntervalDays, WaterAt: sp.WaterAt}
	if defaults.Empty() {
		return bad("This species has no care defaults yet.")
	}
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	now := db.FormatTime(h.auth.Now())
	if err := h.db.InTx(ctx, func(q *dbgen.Queries) error {
		return care.ApplySpecies(ctx, q, plant.ID, defaults, cal.Today, now)
	}); err != nil {
		return nil, err
	}
	items, err := h.plantTasks(ctx, plant.ID, cal)
	if err != nil {
		return nil, err
	}
	return apigen.ApplySpeciesCare200JSONResponse{Items: items}, nil
}

// truncate keeps imported names within what the species form accepts.
func truncate(s string) string {
	if r := []rune(s); len(r) > speciesNameLength {
		return strings.TrimSpace(string(r[:speciesNameLength]))
	}
	return s
}
