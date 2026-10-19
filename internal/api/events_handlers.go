package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

const eventNotFound = "Event not found."

// LogTaskEvent records a done, skip or snooze and returns the task's new schedule.
func (h *Handler) LogTaskEvent(ctx context.Context, req apigen.LogTaskEventRequestObject) (apigen.LogTaskEventResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	in, detail := parseEvent(*req.Body, cal.Today)
	if detail != "" {
		return apigen.LogTaskEvent400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	now := db.FormatTime(h.auth.Now())
	in.ID, in.TaskID, in.OccurredAt, in.CreatedAt = db.NewID(), req.Id, now, now
	row, err := h.db.Writer().CreateEvent(ctx, in)
	switch {
	case db.IsForeignKeyViolation(err):
		return apigen.LogTaskEvent404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, taskNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("log event for task %s: %w", req.Id, err)
	}
	event, err := careEvent(row)
	if err != nil {
		return nil, err
	}
	task, err := h.reloadTask(ctx, req.Id, cal)
	if err != nil {
		return nil, err
	}
	return apigen.LogTaskEvent201JSONResponse{Event: event, Task: task}, nil
}

// UndoEvent voids an event and returns the task's recalculated schedule. Undoing an
// event twice is not an error.
func (h *Handler) UndoEvent(ctx context.Context, req apigen.UndoEventRequestObject) (apigen.UndoEventResponseObject, error) {
	cal, err := h.calendar(ctx)
	if err != nil {
		return nil, err
	}
	taskID, err := h.db.Writer().VoidEvent(ctx, dbgen.VoidEventParams{VoidedAt: sql.NullString{String: db.FormatTime(h.auth.Now()), Valid: true}, ID: req.Id})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UndoEvent404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, eventNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("void event %s: %w", req.Id, err)
	}
	task, err := h.reloadTask(ctx, taskID, cal)
	if err != nil {
		return nil, err
	}
	return apigen.UndoEvent200JSONResponse(task), nil
}

// ListPlantEvents returns a plant's history, newest first, without undone events.
func (h *Handler) ListPlantEvents(ctx context.Context, req apigen.ListPlantEventsRequestObject) (apigen.ListPlantEventsResponseObject, error) {
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return apigen.ListPlantEvents404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	limit := 50
	if req.Params.Limit != nil {
		limit = min(max(*req.Params.Limit, 1), 200)
	}
	rows, err := h.db.Reader().ListPlantHistory(ctx, dbgen.ListPlantHistoryParams{PlantID: req.Id, Limit: int64(limit)})
	if err != nil {
		return nil, fmt.Errorf("list history of plant %s: %w", req.Id, err)
	}
	items := make([]apigen.HistoryEntry, len(rows))
	for i, r := range rows {
		items[i] = apigen.HistoryEntry{TaskKind: apigen.TaskKind(r.TaskKind), TaskLabel: r.TaskLabel}
		if items[i].Event, err = careEvent(r.CareEvent); err != nil {
			return nil, err
		}
	}
	return apigen.ListPlantEvents200JSONResponse{Items: items}, nil
}

// parseEvent validates input against today, returning a user-facing detail when it is invalid.
func parseEvent(in apigen.CareEventInput, today schedule.Date) (dbgen.CreateEventParams, string) {
	f := dbgen.CreateEventParams{Kind: string(in.Kind), LocalDate: today.String(), Note: deref(in.Note)}
	if in.LocalDate != nil {
		on := schedule.DateOf(in.LocalDate.Time)
		// A day either way allows for a tap just before midnight or a clock slightly off.
		if d := on.Sub(today); d < -1 || d > 1 {
			return f, "That date is too far from today."
		}
		f.LocalDate = on.String()
	}
	switch {
	case !in.Kind.Valid():
		return f, "Kind must be done, skipped or snoozed."
	case in.Kind == apigen.Snoozed && (in.SnoozeUntil == nil || !schedule.DateOf(in.SnoozeUntil.Time).After(today)):
		return f, "Pick a snooze date after today."
	case in.Kind != apigen.Snoozed && in.SnoozeUntil != nil:
		return f, "Only a snooze takes a snooze date."
	case utf8.RuneCountInString(f.Note) > 500:
		return f, "Note must be 500 characters or fewer."
	}
	if in.SnoozeUntil != nil {
		f.SnoozeUntil = sql.NullString{String: schedule.DateOf(in.SnoozeUntil.Time).String(), Valid: true}
	}
	return f, ""
}

// reloadTask reads a task after a write and maps it with its schedule.
func (h *Handler) reloadTask(ctx context.Context, id string, cal care.Calendar) (apigen.CareTask, error) {
	row, err := h.db.Reader().GetTask(ctx, id)
	if err != nil {
		return apigen.CareTask{}, fmt.Errorf("reload task %s: %w", id, err)
	}
	return h.careTask(ctx, row, cal)
}

func careEvent(r dbgen.CareEvent) (apigen.CareEvent, error) {
	occurred, err := db.ParseTime(r.OccurredAt)
	if err != nil {
		return apigen.CareEvent{}, err
	}
	on, err := schedule.Parse(r.LocalDate)
	if err != nil {
		return apigen.CareEvent{}, err
	}
	e := apigen.CareEvent{
		Id: r.ID, TaskId: r.TaskID, Kind: apigen.EventKind(r.Kind), LocalDate: apiDate(on), Note: r.Note, OccurredAt: occurred,
	}
	if r.SnoozeUntil.Valid {
		until, err := schedule.Parse(r.SnoozeUntil.String)
		if err != nil {
			return apigen.CareEvent{}, err
		}
		d := apiDate(until)
		e.SnoozeUntil = &d
	}
	return e, nil
}
