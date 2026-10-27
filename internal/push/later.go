package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

var (
	// ErrNotToday means the reminder being postponed is from another day.
	ErrNotToday = errors.New("only today's reminder can be postponed")
	// ErrTooLate means today's later time has already passed.
	ErrTooLate = errors.New("the later time has passed")
)

// RemindLater queues today's reminder to be sent again at the user's later time. It is
// a second digest job keyed digest-later:<user>:<local date>, so asking twice queues it
// once. It returns when the reminder will be sent.
func RemindLater(ctx context.Context, d *db.DB, userID string, date schedule.Date, now time.Time) (time.Time, error) {
	s, err := d.Reader().GetSettings(ctx, userID)
	if err != nil {
		return time.Time{}, fmt.Errorf("load settings: %w", err)
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load time zone %q: %w", s.Timezone, err)
	}
	if date.Compare(schedule.Today(now, loc)) != 0 {
		return time.Time{}, ErrNotToday
	}
	at, err := FireTime(now, s.LaterLocalTime, loc)
	if err != nil {
		return time.Time{}, err
	}
	if !now.Before(at) {
		return time.Time{}, ErrTooLate
	}
	payload, err := json.Marshal(digestPayload{UserID: userID, Date: date.String(), Later: true})
	if err != nil {
		return time.Time{}, err
	}
	key := "digest-later:" + userID + ":" + date.String()
	stamp := db.FormatTime(now)
	q := d.Writer()
	if err := q.EnqueueJob(ctx, dbgen.EnqueueJobParams{
		ID: db.NewID(), Kind: jobKind, DedupeKey: sql.NullString{String: key, Valid: true},
		RunAfter: db.FormatTime(at), Payload: string(payload), CreatedAt: stamp, UpdatedAt: stamp,
	}); err != nil {
		return time.Time{}, fmt.Errorf("queue later reminder: %w", err)
	}
	// Already queued (a second tap, or another device): report the time it keeps.
	runAfter, err := q.GetJobRunAfter(ctx, sql.NullString{String: key, Valid: true})
	if err != nil {
		return time.Time{}, fmt.Errorf("read later reminder: %w", err)
	}
	return db.ParseTime(runAfter)
}

// withLater offers "Remind me at <later>" on a reminder sent before today's later time.
// A reminder that is itself the postponed one never offers it again.
func withLater(m Message, p digestPayload, later string, now time.Time, loc *time.Location) Message {
	if p.Later {
		return m
	}
	at, err := FireTime(now, later, loc)
	if err != nil || !now.Before(at) {
		return m
	}
	m.Date, m.LaterTime = p.Date, later
	return m
}
