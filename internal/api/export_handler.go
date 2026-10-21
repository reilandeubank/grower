package api

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/light"
	"github.com/reilandeubank/grower/internal/media"
	"github.com/reilandeubank/grower/internal/schedule"
)

// The export format (docs/export-format.md) has its own types rather than reusing the
// API's, so an API change can't silently change a versioned file format.
type (
	exportFile struct {
		Format       string              `json:"format"`
		Version      int                 `json:"version"`
		ExportedAt   time.Time           `json:"exported_at"`
		AppVersion   string              `json:"app_version"`
		Settings     apigen.Settings     `json:"settings"`
		Locations    []exportLocation    `json:"locations"`
		Species      []exportSpecies     `json:"species"`
		Plants       []exportPlant       `json:"plants"`
		Tasks        []exportTask        `json:"tasks"`
		Events       []exportEvent       `json:"events"`
		Photos       []exportPhoto       `json:"photos"`
		Measurements []exportMeasurement `json:"measurements"`
	}
	exportLocation struct {
		ID      string             `json:"id"`
		Name    string             `json:"name"`
		Light   *apigen.LightLevel `json:"light"`
		Windows []apigen.Window    `json:"windows"`
		Notes   string             `json:"notes"`
	}
	exportSpecies struct {
		ID                string               `json:"id"`
		ScientificName    string               `json:"scientific_name"`
		CommonName        string               `json:"common_name"`
		Source            apigen.SpeciesSource `json:"source"`
		SourceRef         string               `json:"source_ref"`
		WaterIntervalDays *int                 `json:"water_interval_days"`
		Light             *apigen.LightLevel   `json:"light"`
		WaterAt           *int                 `json:"water_at"`
		Notes             string               `json:"notes"`
		FetchedAt         *time.Time           `json:"fetched_at"`
		UserEdited        bool                 `json:"user_edited"`
	}
	exportPlant struct {
		ID             string                 `json:"id"`
		Name           string                 `json:"name"`
		Species        string                 `json:"species"`
		SpeciesID      *string                `json:"species_id"`
		LocationID     *string                `json:"location_id"`
		AcquiredOn     *openapi_types.Date    `json:"acquired_on"`
		Notes          string                 `json:"notes"`
		PotType        *apigen.PotType        `json:"pot_type"`
		PotSizeIn      *float64               `json:"pot_size_in"`
		Drainage       *bool                  `json:"drainage"`
		Soil           string                 `json:"soil"`
		Light          *apigen.LightLevel     `json:"light"`
		WindowDistance *apigen.WindowDistance `json:"window_distance"`
		GrowLightHours *int                   `json:"grow_light_hours"`
		CoverPhotoID   *string                `json:"cover_photo_id"`
		ArchivedAt     *time.Time             `json:"archived_at"`
		CreatedAt      time.Time              `json:"created_at"`
	}
	exportTask struct {
		ID            string             `json:"id"`
		PlantID       string             `json:"plant_id"`
		Kind          apigen.TaskKind    `json:"kind"`
		Label         string             `json:"label"`
		IntervalDays  int                `json:"interval_days"`
		Anchor        apigen.Anchor      `json:"anchor"`
		StartOn       openapi_types.Date `json:"start_on"`
		Seasonal      bool               `json:"seasonal"`
		MoistureCheck bool               `json:"moisture_check"`
		WaterAt       int                `json:"water_at"`
	}
	exportEvent struct {
		apigen.CareEvent
		VoidedAt *time.Time `json:"voided_at"`
	}
	// exportMeasurement is a Measurement with the photo's id in place of the photo.
	exportMeasurement struct {
		ID         string              `json:"id"`
		PlantID    string              `json:"plant_id"`
		MeasuredOn openapi_types.Date  `json:"measured_on"`
		HeightIn   *float64            `json:"height_in"`
		WidthIn    *float64            `json:"width_in"`
		Stage      *apigen.GrowthStage `json:"stage"`
		Note       string              `json:"note"`
		PhotoID    *string             `json:"photo_id"`
	}
	exportPhoto struct {
		ID      string    `json:"id"`
		PlantID string    `json:"plant_id"`
		Caption string    `json:"caption"`
		TakenAt time.Time `json:"taken_at"`
		Width   int       `json:"width"`
		Height  int       `json:"height"`
		File    string    `json:"file"`
		Sha256  string    `json:"sha256"`
	}
)

// ExportData streams a zip of export.json and the original photos. The rows are read
// up front in one read transaction (a consistent snapshot); the zip is then written
// straight to the response, so photos are never held in memory.
func (h *Handler) ExportData(ctx context.Context, _ apigen.ExportDataRequestObject) (apigen.ExportDataResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.ExportData401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	data, err := h.exportSnapshot(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return exportResponse{data: data, filename: "grower-export-" + cal.Today.String() + ".zip", media: h.media, log: h.log}, nil
}

func (h *Handler) exportSnapshot(ctx context.Context, userID string) (exportFile, error) {
	f := exportFile{
		Format: "grower-export", Version: 1, ExportedAt: h.auth.Now().UTC().Truncate(time.Second), AppVersion: buildinfo.Version,
	}
	tx, err := h.db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return f, fmt.Errorf("export: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := dbgen.New(tx)

	settings, err := q.GetSettings(ctx, userID)
	if err != nil {
		return f, fmt.Errorf("export settings: %w", err)
	}
	f.Settings = settingsOf(settings)
	windows, err := allWindows(ctx, q)
	if err != nil {
		return f, fmt.Errorf("export windows: %w", err)
	}
	locationOf := func(r dbgen.Location) (exportLocation, error) { return exportLocationOf(r, windows[r.ID]), nil }
	if f.Locations, err = exportRows(ctx, q.ExportLocations, locationOf); err != nil {
		return f, fmt.Errorf("export locations: %w", err)
	}
	if f.Species, err = exportRows(ctx, q.ExportSpecies, exportSpeciesOf); err != nil {
		return f, fmt.Errorf("export species: %w", err)
	}
	if f.Plants, err = exportRows(ctx, q.ExportPlants, exportPlantOf); err != nil {
		return f, fmt.Errorf("export plants: %w", err)
	}
	if f.Tasks, err = exportRows(ctx, q.ExportTasks, exportTaskOf); err != nil {
		return f, fmt.Errorf("export tasks: %w", err)
	}
	if f.Events, err = exportRows(ctx, q.ExportEvents, exportEventOf); err != nil {
		return f, fmt.Errorf("export events: %w", err)
	}
	if f.Photos, err = exportRows(ctx, q.ExportPhotos, exportPhotoOf); err != nil {
		return f, fmt.Errorf("export photos: %w", err)
	}
	if f.Measurements, err = exportRows(ctx, q.ExportMeasurements, exportMeasurementOf); err != nil {
		return f, fmt.Errorf("export measurements: %w", err)
	}
	return f, nil
}

// exportRows runs a query and maps every row.
func exportRows[R, T any](ctx context.Context, query func(context.Context) ([]R, error), mapRow func(R) (T, error)) ([]T, error) {
	rows, err := query(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]T, len(rows))
	for i, r := range rows {
		if out[i], err = mapRow(r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func exportLocationOf(r dbgen.Location, windows []light.Window) exportLocation {
	l := exportLocation{ID: r.ID, Name: r.Name, Windows: apiWindows(windows), Notes: r.Notes}
	if r.Light.Valid {
		lv := apigen.LightLevel(r.Light.String)
		l.Light = &lv
	}
	return l
}

// exportSpeciesOf leaves out raw: it is the provider's payload, not the user's data.
func exportSpeciesOf(r dbgen.Species) (exportSpecies, error) {
	s, err := speciesOf(dbgen.GetSpeciesRow{
		ID: r.ID, ScientificName: r.ScientificName, CommonName: r.CommonName, Source: r.Source, SourceRef: r.SourceRef,
		WaterIntervalDays: r.WaterIntervalDays, Light: r.Light, WaterAt: r.WaterAt, Notes: r.Notes, FetchedAt: r.FetchedAt,
		UserEdited: r.UserEdited,
	})
	return exportSpecies{
		ID: s.Id, ScientificName: s.ScientificName, CommonName: s.CommonName, Source: s.Source, SourceRef: s.SourceRef,
		WaterIntervalDays: s.WaterIntervalDays, Light: s.Light, WaterAt: s.WaterAt, Notes: s.Notes, FetchedAt: s.FetchedAt,
		UserEdited: s.UserEdited,
	}, err
}

func exportPlantOf(r dbgen.Plant) (exportPlant, error) {
	p, err := plant(r, nil, nil, nil)
	if err != nil {
		return exportPlant{}, err
	}
	e := exportPlant{
		ID: p.Id, Name: p.Name, Species: p.Species, SpeciesID: p.SpeciesId, LocationID: p.LocationId, AcquiredOn: p.AcquiredOn, Notes: p.Notes,
		PotType: p.PotType, PotSizeIn: p.PotSizeIn, Drainage: p.Drainage, Soil: p.Soil,
		Light: p.Light, WindowDistance: p.WindowDistance, GrowLightHours: p.GrowLightHours, CreatedAt: p.CreatedAt,
	}
	if r.CoverPhotoID.Valid {
		e.CoverPhotoID = &r.CoverPhotoID.String
	}
	e.ArchivedAt, err = nullTime(r.ArchivedAt)
	return e, err
}

func exportTaskOf(r dbgen.CareTask) (exportTask, error) {
	start, err := schedule.Parse(r.StartOn)
	if err != nil {
		return exportTask{}, err
	}
	return exportTask{
		ID: r.ID, PlantID: r.PlantID, Kind: apigen.TaskKind(r.Kind), Label: r.Label, IntervalDays: int(r.IntervalDays),
		Anchor: apigen.Anchor(r.Anchor), StartOn: apiDate(start), Seasonal: r.Seasonal == 1,
		MoistureCheck: r.MoistureCheck == 1, WaterAt: int(r.WaterAt),
	}, nil
}

func exportEventOf(r dbgen.CareEvent) (exportEvent, error) {
	e, err := careEvent(r)
	if err != nil {
		return exportEvent{}, err
	}
	voided, err := nullTime(r.VoidedAt)
	return exportEvent{CareEvent: e, VoidedAt: voided}, err
}

func exportMeasurementOf(r dbgen.PlantMeasurement) (exportMeasurement, error) {
	m, err := measurement(r, nil)
	e := exportMeasurement{
		ID: m.Id, PlantID: m.PlantId, MeasuredOn: m.MeasuredOn, HeightIn: m.HeightIn, WidthIn: m.WidthIn, Stage: m.Stage, Note: m.Note,
	}
	if r.PhotoID.Valid {
		e.PhotoID = &r.PhotoID.String
	}
	return e, err
}

func exportPhotoOf(r dbgen.Photo) (exportPhoto, error) {
	p, err := photo(r)
	if err != nil {
		return exportPhoto{}, err
	}
	return exportPhoto{
		ID: p.Id, PlantID: p.PlantId, Caption: p.Caption, TakenAt: p.TakenAt, Width: p.Width, Height: p.Height,
		File: exportPhotoPath(r), Sha256: r.Sha256,
	}, nil
}

// exportPhotoPath is the photo's original inside the zip.
func exportPhotoPath(r dbgen.Photo) string {
	if r.Mime == "image/png" {
		return "photos/" + r.ID + ".png"
	}
	return "photos/" + r.ID + ".jpg"
}

func nullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := db.ParseTime(s.String)
	return &t, err
}

// exportResponse writes the zip directly to the client: no pipe or goroutine needed,
// and a client that goes away stops the copy with a write error.
type exportResponse struct {
	data     exportFile
	filename string
	media    *media.Store
	log      *slog.Logger
}

func (e exportResponse) VisitExportDataResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+e.filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	// Many photos over a slow link can outlast the server's write timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(w)
	modified := e.data.ExportedAt
	jw, err := zw.CreateHeader(&zip.FileHeader{Name: "export.json", Method: zip.Deflate, Modified: modified})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(jw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(e.data); err != nil {
		return fmt.Errorf("export: write export.json: %w", err)
	}
	for _, p := range e.data.Photos {
		if err := e.addPhoto(zw, p, modified); err != nil {
			return err
		}
	}
	return zw.Close()
}

// addPhoto stores (not deflates: JPEGs don't compress) one original. A missing file
// is skipped with a warning; export.json still lists the photo.
func (e exportResponse) addPhoto(zw *zip.Writer, p exportPhoto, modified time.Time) error {
	f, _, err := e.media.Open(p.ID, "original")
	if errors.Is(err, media.ErrNotFound) {
		e.log.Warn("export: photo file missing, skipped", "photo_id", p.ID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("export: open photo %s: %w", p.ID, err)
	}
	defer f.Close()
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: p.File, Method: zip.Store, Modified: modified})
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return fmt.Errorf("export: copy photo %s: %w", p.ID, err)
	}
	return nil
}
