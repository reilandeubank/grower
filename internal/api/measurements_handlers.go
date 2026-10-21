package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

const (
	measurementNotFound = "Measurement not found."
	unknownPhoto        = "That photo does not exist."
)

// ListMeasurements returns a plant's measurements, newest first, with their photos.
func (h *Handler) ListMeasurements(ctx context.Context, req apigen.ListMeasurementsRequestObject) (apigen.ListMeasurementsResponseObject, error) {
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return apigen.ListMeasurements404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	rows, err := h.db.Reader().ListPlantMeasurements(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("list measurements of plant %s: %w", req.Id, err)
	}
	// A linked photo is always the plant's own, so one query covers them all.
	photos, err := h.db.Reader().ListPlantPhotos(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("list photos of plant %s: %w", req.Id, err)
	}
	byID := make(map[string]*dbgen.Photo, len(photos))
	for i := range photos {
		byID[photos[i].ID] = &photos[i]
	}
	items := make([]apigen.Measurement, len(rows))
	for i, r := range rows {
		if items[i], err = measurement(r, byID[r.PhotoID.String]); err != nil {
			return nil, err
		}
	}
	return apigen.ListMeasurements200JSONResponse{Items: items}, nil
}

// CreateMeasurement logs a plant's size, stage, note or photo on a day.
func (h *Handler) CreateMeasurement(ctx context.Context, req apigen.CreateMeasurementRequestObject) (apigen.CreateMeasurementResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	in, detail := parseMeasurement(*req.Body, cal.Today)
	if detail != "" {
		return apigen.CreateMeasurement400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	var row dbgen.PlantMeasurement
	var ph *dbgen.Photo
	// One write transaction, so the photo can't be deleted between the check and the insert.
	err = h.db.InTx(ctx, func(q *dbgen.Queries) error {
		_, err := q.GetPlant(ctx, req.Id)
		if err != nil {
			return err
		}
		if ph, detail, err = plantPhoto(ctx, q, req.Id, in.PhotoID); err != nil || detail != "" {
			return err
		}
		now := db.FormatTime(h.auth.Now())
		row, err = q.CreateMeasurement(ctx, dbgen.CreateMeasurementParams{
			ID: db.NewID(), PlantID: req.Id, MeasuredOn: in.MeasuredOn, HeightIn: in.HeightIn, WidthIn: in.WidthIn,
			Stage: in.Stage, Note: in.Note, PhotoID: in.PhotoID, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.CreateMeasurement404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("create measurement: %w", err)
	case detail != "":
		return apigen.CreateMeasurement400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	m, err := measurement(row, ph)
	if err != nil {
		return nil, err
	}
	return apigen.CreateMeasurement201JSONResponse(m), nil
}

// UpdateMeasurement replaces a measurement's fields.
func (h *Handler) UpdateMeasurement(ctx context.Context, req apigen.UpdateMeasurementRequestObject) (apigen.UpdateMeasurementResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	in, detail := parseMeasurement(*req.Body, cal.Today)
	if detail != "" {
		return apigen.UpdateMeasurement400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	var row dbgen.PlantMeasurement
	var ph *dbgen.Photo
	err = h.db.InTx(ctx, func(q *dbgen.Queries) error {
		old, err := q.GetMeasurement(ctx, req.Id)
		if err != nil {
			return err
		}
		if ph, detail, err = plantPhoto(ctx, q, old.PlantID, in.PhotoID); err != nil || detail != "" {
			return err
		}
		in.UpdatedAt, in.ID = db.FormatTime(h.auth.Now()), req.Id
		row, err = q.UpdateMeasurement(ctx, in)
		return err
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UpdateMeasurement404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, measurementNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("update measurement %s: %w", req.Id, err)
	case detail != "":
		return apigen.UpdateMeasurement400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	m, err := measurement(row, ph)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMeasurement200JSONResponse(m), nil
}

// DeleteMeasurement removes a measurement; a linked photo stays.
func (h *Handler) DeleteMeasurement(ctx context.Context, req apigen.DeleteMeasurementRequestObject) (apigen.DeleteMeasurementResponseObject, error) {
	n, err := h.db.Writer().DeleteMeasurement(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete measurement %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeleteMeasurement404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, measurementNotFound)), nil
	}
	return apigen.DeleteMeasurement204Response{}, nil
}

// plantPhoto loads the photo to link, if any, returning a user-facing detail when it
// is not one of the plant's own.
func plantPhoto(ctx context.Context, q *dbgen.Queries, plantID string, id sql.NullString) (*dbgen.Photo, string, error) {
	if !id.Valid {
		return nil, "", nil
	}
	ph, err := q.GetPhoto(ctx, id.String)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, unknownPhoto, nil
	case err != nil:
		return nil, "", fmt.Errorf("get photo %s: %w", id.String, err)
	case ph.PlantID != plantID:
		return nil, otherPlantPhoto, nil
	}
	return &ph, "", nil
}

// parseMeasurement validates input, returning a user-facing detail when it is invalid.
// Which photo it links is checked against the plant separately.
func parseMeasurement(in apigen.MeasurementInput, today schedule.Date) (dbgen.UpdateMeasurementParams, string) {
	f := dbgen.UpdateMeasurementParams{Note: strings.TrimSpace(deref(in.Note)), MeasuredOn: today.String()}
	size := func(v *float64) bool { return v == nil || (*v >= 0.1 && *v <= 600) }
	switch {
	case !size(in.HeightIn) || !size(in.WidthIn):
		return f, "Sizes must be 0.1 to 600 inches."
	case in.Stage != nil && !in.Stage.Valid():
		return f, "Stage must be seedling, young or mature."
	case utf8.RuneCountInString(f.Note) > 2000:
		return f, "Note must be 2000 characters or fewer."
	case in.HeightIn == nil && in.WidthIn == nil && in.Stage == nil && f.Note == "" && in.PhotoId == nil:
		return f, "Add a size, a stage, a note or a photo."
	case in.MeasuredOn != nil && schedule.DateOf(in.MeasuredOn.Time).After(today):
		return f, "Pick today or an earlier date."
	}
	if in.MeasuredOn != nil {
		f.MeasuredOn = schedule.DateOf(in.MeasuredOn.Time).String()
	}
	if in.HeightIn != nil {
		f.HeightIn = sql.NullFloat64{Float64: *in.HeightIn, Valid: true}
	}
	if in.WidthIn != nil {
		f.WidthIn = sql.NullFloat64{Float64: *in.WidthIn, Valid: true}
	}
	if in.Stage != nil {
		f.Stage = sql.NullString{String: string(*in.Stage), Valid: true}
	}
	if in.PhotoId != nil {
		f.PhotoID = sql.NullString{String: *in.PhotoId, Valid: true}
	}
	return f, ""
}

// measurement maps a row with its linked photo (nil when it has none).
func measurement(r dbgen.PlantMeasurement, ph *dbgen.Photo) (apigen.Measurement, error) {
	on, err := storedDate(r.MeasuredOn)
	if err != nil {
		return apigen.Measurement{}, err
	}
	m := apigen.Measurement{
		Id: r.ID, PlantId: r.PlantID, MeasuredOn: on, Note: r.Note,
		HeightIn: nullFloat(r.HeightIn), WidthIn: nullFloat(r.WidthIn), Stage: nullStage(r.Stage),
	}
	if ph != nil {
		p, err := photo(*ph)
		if err != nil {
			return apigen.Measurement{}, err
		}
		m.Photo = &p
	}
	return m, nil
}

// growth is a plant's growth summary: apigen.Plant.Growth, an unnamed type there.
type growth = struct {
	HeightIn   *float64            `json:"height_in"`
	MeasuredOn openapi_types.Date  `json:"measured_on"`
	Stage      *apigen.GrowthStage `json:"stage"`
	WidthIn    *float64            `json:"width_in"`
}

// growths returns the growth summary by plant id of one plant or, with plantID unset,
// every plant, in one query. Plants without measurements are absent.
func (h *Handler) growths(ctx context.Context, plantID sql.NullString) (map[string]*growth, error) {
	rows, err := h.db.Reader().ListGrowth(ctx, plantID)
	if err != nil {
		return nil, fmt.Errorf("list growth: %w", err)
	}
	m := make(map[string]*growth, len(rows))
	for _, r := range rows {
		on, err := storedDate(r.MeasuredOn)
		if err != nil {
			return nil, err
		}
		m[r.PlantID] = &growth{HeightIn: nullFloat(r.HeightIn), WidthIn: nullFloat(r.WidthIn), Stage: nullStage(r.Stage), MeasuredOn: on}
	}
	return m, nil
}

func storedDate(s string) (openapi_types.Date, error) {
	d, err := schedule.Parse(s)
	if err != nil {
		return openapi_types.Date{}, fmt.Errorf("parse stored date %q: %w", s, err)
	}
	return apiDate(d), nil
}

func nullFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullStage(v sql.NullString) *apigen.GrowthStage {
	if !v.Valid {
		return nil
	}
	s := apigen.GrowthStage(v.String)
	return &s
}
