package api

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

// writeExport inserts a validated export in one transaction. Rows keep their ids and
// exported timestamps; created_at comes from the id where the export has none, and
// updated_at is the import time. Values are checked by the schema's constraints.
func writeExport(ctx context.Context, d *db.DB, x exportFile, photos map[string]savedPhoto, userID string, now time.Time) error {
	at := db.FormatTime(now)
	return d.InTx(ctx, func(q *dbgen.Queries) error {
		// Again inside the transaction, in case the server wrote something meanwhile.
		if n, err := q.CountContent(ctx); err != nil {
			return fmt.Errorf("count existing data: %w", err)
		} else if n > 0 {
			return ErrNotEmpty
		}
		cur, err := q.GetSettings(ctx, userID)
		if err != nil {
			return fmt.Errorf("import settings: %w", err)
		}
		s := x.Settings
		if _, err := q.UpdateSettings(ctx, dbgen.UpdateSettingsParams{
			Timezone: cmp.Or(s.Timezone, cur.Timezone), DigestLocalTime: cmp.Or(s.DigestTime, cur.DigestLocalTime),
			LaterLocalTime: cmp.Or(s.LaterTime, cur.LaterLocalTime),
			Hemisphere:     cmp.Or(string(s.Hemisphere), cur.Hemisphere), UpdatedAt: at, UserID: userID,
		}); err != nil {
			return fmt.Errorf("import settings: %w", err)
		}
		for _, step := range []func(context.Context, *dbgen.Queries, exportFile, string) error{
			importSpecies, importLocations, importPlants,
		} {
			if err := step(ctx, q, x, at); err != nil {
				return err
			}
		}
		if err := importPhotos(ctx, q, x, photos, at); err != nil {
			return err
		}
		for _, step := range []func(context.Context, *dbgen.Queries, exportFile, string) error{
			importTasks, importEvents, importMeasurements,
		} {
			if err := step(ctx, q, x, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func importSpecies(ctx context.Context, q *dbgen.Queries, x exportFile, at string) error {
	for _, s := range x.Species {
		if err := q.CreateSpecies(ctx, dbgen.CreateSpeciesParams{
			ID: s.ID, ScientificName: s.ScientificName, CommonName: s.CommonName, Source: string(s.Source), SourceRef: s.SourceRef,
			WaterIntervalDays: nullInt(s.WaterIntervalDays), Light: nullEnum(s.Light), WaterAt: nullInt(s.WaterAt), Notes: s.Notes,
			Raw: "{}", FetchedAt: nullTimeString(s.FetchedAt), UserEdited: boolInt(s.UserEdited), CreatedAt: createdAt(s.ID), UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("import species %s: %w", s.ID, err)
		}
	}
	return nil
}

func importLocations(ctx context.Context, q *dbgen.Queries, x exportFile, at string) error {
	for _, l := range x.Locations {
		err := q.CreateLocation(ctx, dbgen.CreateLocationParams{
			ID: l.ID, Name: l.Name, Light: nullEnum(l.Light), Notes: l.Notes, CreatedAt: createdAt(l.ID), UpdatedAt: at,
		})
		for _, w := range l.Windows {
			if err == nil {
				err = q.CreateLocationWindow(ctx, dbgen.CreateLocationWindowParams{LocationID: l.ID, Direction: string(w.Direction), Count: int64(w.Count)})
			}
		}
		if err != nil {
			return fmt.Errorf("import location %s: %w", l.ID, err)
		}
	}
	return nil
}

// importPlants leaves covers out: they point at photos, which point at plants.
func importPlants(ctx context.Context, q *dbgen.Queries, x exportFile, at string) error {
	for _, p := range x.Plants {
		var acquired sql.NullString
		if p.AcquiredOn != nil {
			acquired = sql.NullString{String: p.AcquiredOn.String(), Valid: true}
		}
		var potSize sql.NullFloat64
		if p.PotSizeIn != nil {
			potSize = sql.NullFloat64{Float64: *p.PotSizeIn, Valid: true}
		}
		var drainage sql.NullInt64
		if p.Drainage != nil {
			drainage = sql.NullInt64{Int64: boolInt(*p.Drainage), Valid: true}
		}
		if err := q.ImportPlant(ctx, dbgen.ImportPlantParams{
			ID: p.ID, Name: p.Name, Species: p.Species, SpeciesID: nullEnum(p.SpeciesID), LocationID: nullEnum(p.LocationID),
			AcquiredOn: acquired, Notes: p.Notes, PotType: nullEnum(p.PotType), PotSizeIn: potSize, Drainage: drainage, Soil: p.Soil,
			Light: nullEnum(p.Light), WindowDistance: nullEnum(p.WindowDistance), GrowLightHours: nullInt(p.GrowLightHours),
			ArchivedAt: nullTimeString(p.ArchivedAt), CreatedAt: db.FormatTime(p.CreatedAt), UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("import plant %s: %w", p.ID, err)
		}
	}
	return nil
}

// importPhotos inserts the photos, described from their stored files, then the covers.
func importPhotos(ctx context.Context, q *dbgen.Queries, x exportFile, photos map[string]savedPhoto, at string) error {
	for _, p := range x.Photos {
		s := photos[p.ID]
		if _, err := q.CreatePhoto(ctx, dbgen.CreatePhotoParams{
			ID: p.ID, PlantID: p.PlantID, TakenAt: db.FormatTime(p.TakenAt), Caption: p.Caption, Mime: s.image.MIME,
			Width: int64(s.image.Width), Height: int64(s.image.Height), Bytes: int64(s.bytes), Sha256: p.Sha256,
			CreatedAt: createdAt(p.ID), UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("import photo %s: %w", p.ID, err)
		}
	}
	for _, p := range x.Plants {
		if p.CoverPhotoID == nil {
			continue
		}
		if _, err := q.SetPlantCover(ctx, dbgen.SetPlantCoverParams{CoverPhotoID: nullEnum(p.CoverPhotoID), UpdatedAt: at, ID: p.ID}); err != nil {
			return fmt.Errorf("import cover of plant %s: %w", p.ID, err)
		}
	}
	return nil
}

func importTasks(ctx context.Context, q *dbgen.Queries, x exportFile, at string) error {
	for _, t := range x.Tasks {
		if _, err := q.CreateTask(ctx, dbgen.CreateTaskParams{
			ID: t.ID, PlantID: t.PlantID, Kind: string(t.Kind), Label: t.Label, IntervalDays: int64(t.IntervalDays),
			Anchor: string(t.Anchor), StartOn: t.StartOn.String(), Seasonal: boolInt(t.Seasonal), MoistureCheck: boolInt(t.MoistureCheck),
			WaterAt: int64(t.WaterAt), CreatedAt: createdAt(t.ID), UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("import task %s: %w", t.ID, err)
		}
	}
	return nil
}

func importEvents(ctx context.Context, q *dbgen.Queries, x exportFile, _ string) error {
	for _, e := range x.Events {
		var snooze sql.NullString
		if e.SnoozeUntil != nil {
			snooze = sql.NullString{String: e.SnoozeUntil.String(), Valid: true}
		}
		var reading sql.NullInt64
		if e.Reading != nil {
			reading = sql.NullInt64{Int64: int64(*e.Reading), Valid: true}
		}
		if err := q.ImportEvent(ctx, dbgen.ImportEventParams{
			ID: e.Id, TaskID: e.TaskId, Kind: string(e.Kind), LocalDate: e.LocalDate.String(), SnoozeUntil: snooze, Reading: reading,
			Note: e.Note, OccurredAt: db.FormatTime(e.OccurredAt), VoidedAt: nullTimeString(e.VoidedAt), CreatedAt: createdAt(e.Id),
		}); err != nil {
			return fmt.Errorf("import event %s: %w", e.Id, err)
		}
	}
	return nil
}

func importMeasurements(ctx context.Context, q *dbgen.Queries, x exportFile, at string) error {
	for _, m := range x.Measurements {
		var height, width sql.NullFloat64
		if m.HeightIn != nil {
			height = sql.NullFloat64{Float64: *m.HeightIn, Valid: true}
		}
		if m.WidthIn != nil {
			width = sql.NullFloat64{Float64: *m.WidthIn, Valid: true}
		}
		if _, err := q.CreateMeasurement(ctx, dbgen.CreateMeasurementParams{
			ID: m.ID, PlantID: m.PlantID, MeasuredOn: m.MeasuredOn.String(), HeightIn: height, WidthIn: width,
			Stage: nullEnum(m.Stage), Note: m.Note, PhotoID: nullEnum(m.PhotoID), CreatedAt: createdAt(m.ID), UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("import measurement %s: %w", m.ID, err)
		}
	}
	return nil
}

// nullEnum stores an optional string or string enum.
func nullEnum[T ~string](p *T) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*p), Valid: true}
}

func nullTimeString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: db.FormatTime(*t), Valid: true}
}
