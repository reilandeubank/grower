package care

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

// Defaults for a water task created from a species that lacks one of them.
const (
	defaultWaterInterval = 7
	defaultWaterAt       = 3
)

// SpeciesDefaults are a species' care defaults; invalid fields are unknown.
type SpeciesDefaults struct {
	WaterIntervalDays sql.NullInt64
	WaterAt           sql.NullInt64
}

// Empty reports whether there is nothing to apply.
func (s SpeciesDefaults) Empty() bool { return !s.WaterIntervalDays.Valid && !s.WaterAt.Valid }

// ApplySpecies puts a species' defaults on the plant's first water task: the interval
// if the species has one, and water_at if it has one and the task is a soil check.
// A plant without a water task gets a plain one starting today. Run it in a transaction.
func ApplySpecies(ctx context.Context, q *dbgen.Queries, plantID string, s SpeciesDefaults, today schedule.Date, now string) error {
	task, err := q.FirstWaterTask(ctx, plantID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = q.CreateTask(ctx, dbgen.CreateTaskParams{
			ID: db.NewID(), PlantID: plantID, Kind: "water", IntervalDays: orInt(s.WaterIntervalDays, defaultWaterInterval),
			Anchor: "from_completion", StartOn: today.String(), Seasonal: 1, MoistureCheck: 0,
			WaterAt: orInt(s.WaterAt, defaultWaterAt), CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("create water task for plant %s: %w", plantID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("first water task of plant %s: %w", plantID, err)
	}
	task.IntervalDays = orInt(s.WaterIntervalDays, task.IntervalDays)
	if task.MoistureCheck == 1 {
		task.WaterAt = orInt(s.WaterAt, task.WaterAt)
	}
	_, err = q.UpdateTask(ctx, dbgen.UpdateTaskParams{
		Kind: task.Kind, Label: task.Label, IntervalDays: task.IntervalDays, Anchor: task.Anchor, StartOn: task.StartOn,
		Seasonal: task.Seasonal, MoistureCheck: task.MoistureCheck, WaterAt: task.WaterAt, UpdatedAt: now, ID: task.ID,
	})
	if err != nil {
		return fmt.Errorf("update water task %s: %w", task.ID, err)
	}
	return nil
}

func orInt(v sql.NullInt64, def int64) int64 {
	if v.Valid {
		return v.Int64
	}
	return def
}
