package push

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/db/dbtest"
	"github.com/reilandeubank/grower/internal/push/pushtest"
)

var chicago, _ = time.LoadLocation("America/Chicago")

type fixture struct {
	t      *testing.T
	digest *Digest
	fake   *pushtest.Service
	now    time.Time
	userID string
}

// newFixture has one user in tz with the reminder at clock, one subscribed device
// (/device-1) and Monstera, whose watering is due 2026-03-01 and every 30 days after.
func newFixture(t *testing.T, tz, clock string) *fixture {
	t.Helper()
	d := dbtest.Open(t)
	keys, err := LoadKeys(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, fake: pushtest.New(t), userID: db.NewID()}
	now := func() time.Time { return f.now }
	log := slog.New(slog.DiscardHandler)
	sender := &Sender{DB: d, Keys: keys, Subject: "https://grower.test", Client: f.fake.Client(), Now: now, Log: log}
	f.digest = &Digest{DB: d, Sender: sender, Now: now, Log: log}

	ctx, q, stamp := context.Background(), d.Writer(), "2026-01-01T00:00:00Z"
	must(t, q.CreateUser(ctx, dbgen.CreateUserParams{ID: f.userID, Username: "fern", PasswordHash: "x", CreatedAt: stamp, UpdatedAt: stamp}))
	must(t, q.CreateSettings(ctx, dbgen.CreateSettingsParams{UserID: f.userID, Timezone: tz, CreatedAt: stamp, UpdatedAt: stamp}))
	_, err = q.UpdateSettings(ctx, dbgen.UpdateSettingsParams{Timezone: tz, DigestLocalTime: clock, Hemisphere: "north", UpdatedAt: stamp, UserID: f.userID})
	must(t, err)
	f.addPlant("Monstera", "water", "2026-03-01")
	f.subscribe("/device-1")
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) addPlant(name, kind, start string) {
	f.t.Helper()
	ctx, q, stamp := context.Background(), f.digest.DB.Writer(), "2026-01-01T00:00:00Z"
	p, err := q.CreatePlant(ctx, dbgen.CreatePlantParams{ID: db.NewID(), Name: name, CreatedAt: stamp, UpdatedAt: stamp})
	must(f.t, err)
	_, err = q.CreateTask(ctx, dbgen.CreateTaskParams{
		ID: db.NewID(), PlantID: p.ID, Kind: kind, Label: "", IntervalDays: 30, Anchor: "fixed", StartOn: start, WaterAt: 3, CreatedAt: stamp, UpdatedAt: stamp,
	})
	must(f.t, err)
}

func (f *fixture) subscribe(path string) {
	f.t.Helper()
	p256dh, auth := pushtest.Keys(f.t)
	must(f.t, f.digest.DB.Writer().UpsertPushSubscription(context.Background(), dbgen.UpsertPushSubscriptionParams{
		ID: db.NewID(), UserID: f.userID, Endpoint: f.fake.URL + path, P256dh: p256dh, Auth: auth, CreatedAt: "2026-01-01T00:00:00Z",
	}))
}

// tick runs the digest at the given time and returns how many pushes were sent in all.
func (f *fixture) tick(at time.Time) int {
	f.t.Helper()
	f.now = at
	must(f.t, f.digest.Tick(context.Background()))
	return len(f.fake.Requests())
}

type jobRow struct {
	Status, RunAfter string
	Attempts         int
	Note             sql.NullString
}

func (f *fixture) jobs() []jobRow {
	f.t.Helper()
	rows, err := f.digest.DB.Read.Query(`SELECT status, run_after, attempts, last_error FROM jobs ORDER BY created_at, id`)
	must(f.t, err)
	defer rows.Close()
	var out []jobRow
	for rows.Next() {
		var j jobRow
		must(f.t, rows.Scan(&j.Status, &j.RunAfter, &j.Attempts, &j.Note))
		out = append(out, j)
	}
	must(f.t, rows.Err())
	return out
}

func TestDigestFiresOncePerLocalDay(t *testing.T) {
	for _, tz := range []string{"America/Chicago", "Asia/Tokyo", "Pacific/Auckland", "UTC"} {
		t.Run(tz, func(t *testing.T) {
			f := newFixture(t, tz, "08:00")
			loc, _ := time.LoadLocation(tz)
			at := func(day, h, m int) time.Time { return time.Date(2026, 3, day, h, m, 0, 0, loc) }
			steps := []struct {
				at   time.Time
				want int
			}{
				{at(1, 7, 59), 0},
				{at(1, 8, 0), 1},
				{at(1, 8, 1), 1},
				{at(1, 23, 59), 1},
				{at(2, 7, 59), 1},
				{at(2, 8, 0), 2}, // overdue counts too
				{at(2, 12, 0), 2},
			}
			for _, s := range steps {
				if got := f.tick(s.at); got != s.want {
					t.Fatalf("at %s: %d pushes, want %d", s.at, got, s.want)
				}
			}
			// A restart (a new runner on the same database) sends nothing new.
			f.digest = &Digest{DB: f.digest.DB, Sender: f.digest.Sender, Now: f.digest.Now, Log: f.digest.Log}
			if got := f.tick(at(2, 9, 0)); got != 2 {
				t.Errorf("after restart: %d pushes, want 2", got)
			}
			if got := len(f.jobs()); got != 2 {
				t.Errorf("%d jobs, want one per day", got)
			}
		})
	}
}

func TestDigestAcrossDST(t *testing.T) {
	// Chicago springs forward 2026-03-08 02:00 and falls back 2026-11-01 02:00.
	f := newFixture(t, "America/Chicago", "08:00")
	if got := f.tick(time.Date(2026, 3, 8, 12, 59, 0, 0, time.UTC)); got != 0 { // 07:59 CDT
		t.Fatalf("before 08:00 CDT: %d pushes", got)
	}
	if got := f.tick(time.Date(2026, 3, 8, 13, 0, 0, 0, time.UTC)); got != 1 {
		t.Fatalf("at 08:00 CDT: %d pushes", got)
	}

	for _, c := range []struct {
		date  time.Time
		clock string
		want  time.Time
	}{
		// 02:30 does not exist on spring-forward day; it moves to 03:30 CDT.
		{time.Date(2026, 3, 8, 12, 0, 0, 0, chicago), "02:30", time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC)},
		// 01:30 happens twice on fall-back day; the first (CDT) counts.
		{time.Date(2026, 11, 1, 12, 0, 0, 0, chicago), "01:30", time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC)},
	} {
		got, err := FireTime(c.date, c.clock, chicago)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("FireTime(%s, %s) = %s, %v; want %s", c.date, c.clock, got.UTC(), err, c.want)
		}
	}

	f = newFixture(t, "America/Chicago", "01:30")
	f.tick(time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC))                       // 01:30 CDT
	if got := f.tick(time.Date(2026, 11, 1, 7, 30, 0, 0, time.UTC)); got != 1 { // 01:30 CST
		t.Errorf("fall-back day: %d pushes, want 1", got)
	}
}

func TestDigestNothingDue(t *testing.T) {
	f := newFixture(t, "UTC", "08:00")
	if got := f.tick(time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)); got != 0 { // Monstera starts Mar 1
		t.Fatalf("%d pushes with nothing due", got)
	}
	if j := f.jobs(); len(j) != 1 || j[0].Status != "done" || j[0].Note.String != "nothing due" {
		t.Errorf("jobs = %+v", j)
	}
}

func TestDigestRetriesWithBackoff(t *testing.T) {
	f := newFixture(t, "UTC", "08:00")
	f.fake.SetStatus("/device-1", http.StatusInternalServerError)
	start := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	// Attempts at +0, then 1, 2, 4 and 8 minutes after each failure.
	steps := []struct {
		after time.Duration
		want  int
	}{
		{0, 1}, {30 * time.Second, 1}, {time.Minute, 2}, {2 * time.Minute, 2},
		{3 * time.Minute, 3}, {7 * time.Minute, 4}, {14 * time.Minute, 4}, {15 * time.Minute, 5}, {time.Hour, 5},
	}
	for _, s := range steps {
		if got := f.tick(start.Add(s.after)); got != s.want {
			t.Fatalf("at +%s: %d attempts, want %d", s.after, got, s.want)
		}
	}
	if j := f.jobs(); len(j) != 1 || j[0].Status != "failed" || j[0].Attempts != 5 {
		t.Errorf("jobs = %+v", j)
	}
	var failed int
	must(t, f.digest.DB.Read.QueryRow(`SELECT failed_count FROM push_subscriptions`).Scan(&failed))
	if failed != 5 {
		t.Errorf("failed_count = %d, want 5", failed)
	}
}

func TestDigestRecoversAfterAFailure(t *testing.T) {
	f := newFixture(t, "UTC", "08:00")
	f.fake.SetStatus("/device-1", http.StatusServiceUnavailable)
	start := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	f.tick(start)
	f.fake.SetStatus("/device-1", http.StatusCreated)
	if got := f.tick(start.Add(time.Minute)); got != 2 {
		t.Fatalf("%d requests, want 2", got)
	}
	if j := f.jobs(); j[0].Status != "done" || j[0].Attempts != 1 {
		t.Errorf("jobs = %+v", j)
	}
	var failed int
	var lastOK sql.NullString
	must(t, f.digest.DB.Read.QueryRow(`SELECT failed_count, last_ok_at FROM push_subscriptions`).Scan(&failed, &lastOK))
	if failed != 0 || lastOK.String != "2026-03-01T08:01:00Z" {
		t.Errorf("failed_count = %d, last_ok_at = %v", failed, lastOK)
	}
}

func TestDigestSkipsADayThatEnded(t *testing.T) {
	f := newFixture(t, "America/Chicago", "08:00")
	f.fake.SetStatus("/device-1", http.StatusInternalServerError)
	f.tick(time.Date(2026, 3, 1, 23, 59, 30, 0, chicago)) // attempt 1 fails, retry at 00:00:30
	f.fake.SetStatus("/device-1", http.StatusCreated)
	if got := f.tick(time.Date(2026, 3, 2, 0, 1, 0, 0, chicago)); got != 1 {
		t.Fatalf("yesterday's reminder was sent: %d requests", got)
	}
	if j := f.jobs(); len(j) != 1 || j[0].Status != "done" || j[0].Note.String != "skipped: the day ended before it was sent" {
		t.Errorf("jobs = %+v", j)
	}
}

func TestDigestReclaimsAnExpiredLock(t *testing.T) {
	f := newFixture(t, "UTC", "08:00")
	start := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	f.now = start
	must(t, f.digest.enqueue(context.Background()))
	// A crash mid-send: claimed but never finished.
	_, err := f.digest.DB.Write.Exec(`UPDATE jobs SET status = 'running', locked_until = ?`, db.FormatTime(start.Add(lockFor)))
	must(t, err)
	if got := f.tick(start.Add(lockFor - time.Second)); got != 0 {
		t.Fatalf("a locked job was taken: %d", got)
	}
	if got := f.tick(start.Add(lockFor)); got != 1 {
		t.Fatalf("expired lock not reclaimed: %d", got)
	}
}

func TestSendRemovesGoneSubscriptions(t *testing.T) {
	f := newFixture(t, "UTC", "08:00")
	f.subscribe("/device-2")
	f.fake.SetStatus("/device-1", http.StatusGone)
	f.fake.SetStatus("/device-2", http.StatusNotFound)
	f.tick(time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC))
	var n int
	must(t, f.digest.DB.Read.QueryRow(`SELECT count(*) FROM push_subscriptions`).Scan(&n))
	if n != 0 {
		t.Errorf("%d subscriptions left, want 0", n)
	}
	if j := f.jobs(); j[0].Status != "done" {
		t.Errorf("jobs = %+v", j)
	}
}

func TestLoadKeysPersists(t *testing.T) {
	dir := t.TempDir()
	k1, err := LoadKeys(dir)
	must(t, err)
	k2, err := LoadKeys(dir)
	must(t, err)
	if k1 != k2 || k1.Public == "" {
		t.Errorf("keys changed between starts: %+v vs %+v", k1, k2)
	}
	fi, err := os.Stat(filepath.Join(dir, KeysFile))
	must(t, err)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
}
