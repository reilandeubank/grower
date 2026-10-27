package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/schedule"
)

const (
	jobKind     = "digest"
	tickEvery   = time.Minute
	lockFor     = 5 * time.Minute
	maxAttempts = 5
	// firstRetry doubles with each failed attempt: 1, 2, 4 and 8 minutes.
	firstRetry = time.Minute
)

// Digest sends each user the daily reminder at their digest time. Each day's reminder
// is a jobs row keyed digest:<user>:<local date>, so restarts never queue it twice.
// It is sent before the row is marked done: a crash in between repeats it once, which
// beats losing it (docs/architecture.md, "Auth and operations").
type Digest struct {
	DB     *db.DB
	Sender *Sender
	Now    func() time.Time
	Log    *slog.Logger
}

type digestPayload struct {
	UserID string `json:"user_id"`
	Date   string `json:"date"`
	// Later marks the reminder sent again at the later time (RemindLater).
	Later bool `json:"later,omitempty"`
}

// Run ticks once now and then every minute until ctx is cancelled.
func (d *Digest) Run(ctx context.Context) {
	t := time.NewTicker(tickEvery)
	defer t.Stop()
	for {
		if err := d.Tick(ctx); err != nil && ctx.Err() == nil {
			d.Log.Error("daily reminder failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick queues the reminders that are due and sends every queued one.
func (d *Digest) Tick(ctx context.Context) error {
	if err := d.enqueue(ctx); err != nil {
		return err
	}
	for {
		now := d.Now()
		job, err := d.DB.Writer().ClaimJob(ctx, dbgen.ClaimJobParams{
			JobKind: jobKind, Now: db.FormatTime(now), LockedUntil: sql.NullString{String: db.FormatTime(now.Add(lockFor)), Valid: true},
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim reminder job: %w", err)
		}
		if err := d.finish(ctx, job, d.process(ctx, job)); err != nil {
			return err
		}
	}
}

// enqueue adds today's reminder for every user whose digest time has passed.
func (d *Digest) enqueue(ctx context.Context) error {
	all, err := d.DB.Reader().ListSettings(ctx)
	if err != nil {
		return fmt.Errorf("list settings: %w", err)
	}
	now := d.Now()
	for _, s := range all {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil {
			d.Log.WarnContext(ctx, "skip reminder: bad time zone", "user", s.UserID, "err", err)
			continue
		}
		at, err := FireTime(now, s.DigestLocalTime, loc)
		if err != nil {
			d.Log.WarnContext(ctx, "skip reminder: bad digest time", "user", s.UserID, "err", err)
			continue
		}
		if now.Before(at) {
			continue
		}
		date := schedule.Today(now, loc).String()
		payload, err := json.Marshal(digestPayload{UserID: s.UserID, Date: date})
		if err != nil {
			return err
		}
		stamp := db.FormatTime(now)
		if err := d.DB.Writer().EnqueueJob(ctx, dbgen.EnqueueJobParams{
			ID: db.NewID(), Kind: jobKind, DedupeKey: sql.NullString{String: "digest:" + s.UserID + ":" + date, Valid: true},
			RunAfter: db.FormatTime(at), Payload: string(payload), CreatedAt: stamp, UpdatedAt: stamp,
		}); err != nil {
			return fmt.Errorf("queue reminder for user %s: %w", s.UserID, err)
		}
	}
	return nil
}

// FireTime is when the reminder is due on now's local date. A time in a spring-forward
// gap moves forward by the gap (02:30 becomes 03:30); in a fall-back overlap it is the
// first of the two.
func FireTime(now time.Time, clock string, loc *time.Location) (time.Time, error) {
	hm, err := ParseClock(clock)
	if err != nil {
		return time.Time{}, err
	}
	y, m, day := now.In(loc).Date()
	t := time.Date(y, m, day, hm.Hour(), hm.Minute(), 0, 0, loc)
	// In a gap Go applies the earlier offset, landing that much before the wall time.
	if gap := wallMinutes(hm) - wallMinutes(t); gap > 0 {
		t = t.Add(time.Duration(gap) * time.Minute)
	}
	return t, nil
}

func wallMinutes(t time.Time) int { return t.Hour()*60 + t.Minute() }

// ParseClock parses a 24-hour "HH:MM" time.
func ParseClock(s string) (time.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != len("15:04") {
		return time.Time{}, fmt.Errorf("digest time %q is not HH:MM", s)
	}
	return t, nil
}

// errSkip ends a job without sending; its text is kept as the job's note.
type errSkip string

func (e errSkip) Error() string { return string(e) }

// process sends one reminder, or returns errSkip when there is nothing to send.
func (d *Digest) process(ctx context.Context, job dbgen.Job) error {
	var p digestPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return errSkip("bad payload: " + err.Error())
	}
	date, err := schedule.Parse(p.Date)
	if err != nil {
		return errSkip("bad payload date: " + err.Error())
	}
	settings, err := d.DB.Reader().GetSettings(ctx, p.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return errSkip("user no longer exists")
	}
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return errSkip("bad time zone: " + err.Error())
	}
	// After downtime or long retries, yesterday's "needs care today" would mislead.
	if date.Compare(schedule.Today(d.Now(), loc)) < 0 {
		return errSkip("skipped: the day ended before it was sent")
	}
	agenda, err := care.Today(ctx, d.DB.Reader(), care.Calendar{Today: date, Hemisphere: schedule.Hemisphere(settings.Hemisphere)})
	if err != nil {
		return err
	}
	m, ok := DigestMessage(agenda)
	if !ok {
		return errSkip("nothing due")
	}
	return d.Sender.Send(ctx, p.UserID, withLater(m, p, settings.LaterLocalTime, d.Now(), loc))
}

// finish records the outcome: done (with a note for skips), or another attempt with
// exponential backoff until maxAttempts, then failed.
func (d *Digest) finish(ctx context.Context, job dbgen.Job, result error) error {
	now := d.Now()
	q := d.DB.Writer()
	var skip errSkip
	if result == nil || errors.As(result, &skip) {
		note := sql.NullString{}
		if result != nil {
			note = sql.NullString{String: result.Error(), Valid: true}
		}
		if err := q.FinishJob(ctx, dbgen.FinishJobParams{LastError: note, UpdatedAt: db.FormatTime(now), ID: job.ID}); err != nil {
			return fmt.Errorf("finish reminder job %s: %w", job.ID, err)
		}
		return nil
	}

	attempts := job.Attempts + 1
	status := "pending"
	if attempts >= maxAttempts {
		status = "failed"
	}
	d.Log.ErrorContext(ctx, "daily reminder attempt failed", "job", job.ID, "attempt", attempts, "status", status, "err", result)
	if err := q.FailJob(ctx, dbgen.FailJobParams{
		Status: status, LastError: sql.NullString{String: result.Error(), Valid: true},
		RunAfter: db.FormatTime(now.Add(firstRetry << (attempts - 1))), UpdatedAt: db.FormatTime(now), ID: job.ID,
	}); err != nil {
		return fmt.Errorf("record failed reminder job %s: %w", job.ID, err)
	}
	return nil
}
