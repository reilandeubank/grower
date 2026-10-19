package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

const taskNotFound = "Task not found."

// ListPlantTasks returns a plant's tasks, soonest due first.
func (h *Handler) ListPlantTasks(ctx context.Context, req apigen.ListPlantTasksRequestObject) (apigen.ListPlantTasksResponseObject, error) {
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return apigen.ListPlantTasks404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := h.db.Reader().ListPlantTasks(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("list tasks of plant %s: %w", req.Id, err)
	}
	items := make([]apigen.CareTask, len(rows))
	for i, r := range rows {
		if items[i], err = h.careTask(ctx, r, cal); err != nil {
			return nil, err
		}
	}
	// Rows come ordered by kind, so equal due dates stay in kind order.
	slices.SortStableFunc(items, func(a, b apigen.CareTask) int { return a.NextDue.Compare(b.NextDue.Time) })
	return apigen.ListPlantTasks200JSONResponse{Items: items}, nil
}

// CreateTask adds a care task to a plant.
func (h *Handler) CreateTask(ctx context.Context, req apigen.CreateTaskRequestObject) (apigen.CreateTaskResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	in, detail := parseTask(*req.Body, cal.Today)
	if detail != "" {
		return apigen.CreateTask400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	now := db.FormatTime(h.auth.Now())
	row, err := h.db.Writer().CreateTask(ctx, dbgen.CreateTaskParams{
		ID: db.NewID(), PlantID: req.Id, Kind: in.Kind, Label: in.Label, IntervalDays: in.IntervalDays,
		Anchor: in.Anchor, StartOn: in.StartOn, Seasonal: in.Seasonal, MoistureCheck: in.MoistureCheck, WaterAt: in.WaterAt,
		CreatedAt: now, UpdatedAt: now,
	})
	switch {
	case db.IsForeignKeyViolation(err):
		return apigen.CreateTask404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("create task: %w", err)
	}
	t, err := careTask(row, nil, cal)
	if err != nil {
		return nil, err
	}
	return apigen.CreateTask201JSONResponse(t), nil
}

// UpdateTask replaces a task's schedule; its history stays.
func (h *Handler) UpdateTask(ctx context.Context, req apigen.UpdateTaskRequestObject) (apigen.UpdateTaskResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	in, detail := parseTask(*req.Body, cal.Today)
	if detail != "" {
		return apigen.UpdateTask400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	in.UpdatedAt, in.ID = db.FormatTime(h.auth.Now()), req.Id
	row, err := h.db.Writer().UpdateTask(ctx, in)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UpdateTask404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, taskNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("update task %s: %w", req.Id, err)
	}
	t, err := h.careTask(ctx, row, cal)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateTask200JSONResponse(t), nil
}

// DeleteTask removes a task and, by cascade, its history.
func (h *Handler) DeleteTask(ctx context.Context, req apigen.DeleteTaskRequestObject) (apigen.DeleteTaskResponseObject, error) {
	n, err := h.db.Writer().DeleteTask(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete task %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeleteTask404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, taskNotFound)), nil
	}
	return apigen.DeleteTask204Response{}, nil
}

// parseTask validates input, returning a user-facing detail when it is invalid.
// Whether the plant exists is left to the foreign key.
func parseTask(in apigen.CareTaskInput, today schedule.Date) (dbgen.UpdateTaskParams, string) {
	f := dbgen.UpdateTaskParams{Kind: string(in.Kind), IntervalDays: int64(in.IntervalDays), Anchor: string(apigen.FromCompletion), WaterAt: 3}
	if in.Kind == apigen.Custom {
		f.Label = strings.TrimSpace(deref(in.Label))
	}
	// PUT is a full replace, so omitted soil-check fields reset to their defaults too.
	check := in.MoistureCheck != nil && *in.MoistureCheck
	if in.WaterAt != nil {
		f.WaterAt = int64(*in.WaterAt)
	}
	switch {
	case !in.Kind.Valid():
		return f, "Kind must be water, fertilize, mist, repot, rotate or custom."
	case in.Kind == apigen.Custom && f.Label == "":
		return f, "Label is required for a custom task."
	case utf8.RuneCountInString(f.Label) > 40:
		return f, "Label must be 40 characters or fewer."
	case in.IntervalDays < 1 || in.IntervalDays > 365:
		return f, "Interval must be 1 to 365 days."
	case in.Anchor != nil && !in.Anchor.Valid():
		return f, "Anchor must be from_completion or fixed."
	case f.WaterAt < 1 || f.WaterAt > 9:
		return f, "Water-at must be between 1 and 9."
	case check && in.Kind != apigen.Water:
		return f, "Only watering tasks can use soil checks."
	case check && in.Anchor != nil && *in.Anchor == apigen.Fixed:
		return f, "Soil checks always count from the last watering."
	}
	f.MoistureCheck = boolInt(check)
	if in.Anchor != nil {
		f.Anchor = string(*in.Anchor)
	}
	// PUT is a full replace, so an omitted seasonal resets to the kind's default there too.
	seasonal := in.Kind == apigen.Water || in.Kind == apigen.Mist || in.Kind == apigen.Fertilize
	if in.Seasonal != nil {
		seasonal = *in.Seasonal
	}
	f.Seasonal = boolInt(seasonal)
	f.StartOn = today.String()
	if in.StartOn != nil {
		f.StartOn = schedule.DateOf(in.StartOn.Time).String()
	}
	return f, ""
}

// careTask loads a task's events and maps it with its computed schedule.
func (h *Handler) careTask(ctx context.Context, row dbgen.CareTask, cal care.Calendar) (apigen.CareTask, error) {
	events, err := h.db.Reader().ListTaskEvents(ctx, row.ID)
	if err != nil {
		return apigen.CareTask{}, fmt.Errorf("list events of task %s: %w", row.ID, err)
	}
	return careTask(row, events, cal)
}

// careTask maps a task, computing next_due and last_done from its non-voided events
// in insertion order.
func careTask(row dbgen.CareTask, events []dbgen.CareEvent, cal care.Calendar) (apigen.CareTask, error) {
	dates, err := care.Schedule(row, events, cal)
	if err != nil {
		return apigen.CareTask{}, err
	}
	return taskOf(row, dates), nil
}

// taskOf maps a task with its computed schedule.
func taskOf(row dbgen.CareTask, dates care.Dates) apigen.CareTask {
	t := apigen.CareTask{
		Id: row.ID, PlantId: row.PlantID, Kind: apigen.TaskKind(row.Kind), Label: row.Label,
		IntervalDays: int(row.IntervalDays), Anchor: apigen.Anchor(row.Anchor),
		StartOn: apiDate(dates.Start), NextDue: apiDate(dates.Next), Seasonal: row.Seasonal == 1, CurrentIntervalDays: dates.Interval,
		MoistureCheck: row.MoistureCheck == 1, WaterAt: int(row.WaterAt),
	}
	if dates.HasLast {
		d := apiDate(dates.Last)
		t.LastDone = &d
	}
	if dates.HasDrying {
		t.DryingDays = &dates.DryingDays
	}
	if r := dates.LastReading; r.Reading > 0 {
		t.LastReading = &struct {
			On    openapi_types.Date     `json:"on"`
			Value apigen.MoistureReading `json:"value"`
		}{On: apiDate(r.On), Value: r.Reading}
	}
	return t
}

// calendar is today in the signed-in user's time zone, with their hemisphere.
func (h *Handler) calendar(ctx context.Context) (care.Calendar, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return care.Calendar{}, errors.New("no signed-in user")
	}
	settings, err := h.db.Reader().GetSettings(ctx, p.UserID)
	if err != nil {
		return care.Calendar{}, fmt.Errorf("load settings for user %s: %w", p.UserID, err)
	}
	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return care.Calendar{}, fmt.Errorf("user %s time zone: %w", p.UserID, err)
	}
	return care.Calendar{Today: schedule.Today(h.auth.Now(), loc), Hemisphere: schedule.Hemisphere(settings.Hemisphere)}, nil
}

func apiDate(d schedule.Date) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)}
}
