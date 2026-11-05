package care

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/schedule"
)

// history fills a database with plants whose tasks each get a random event log, and
// returns the open database and its task ids. Without everyday, logs are adversarial:
// many events share a day, which makes same-day drying cycles common.
func history(t testing.TB, seed uint64, plants, tasksPerPlant, events int, everyday bool) (*db.DB, []string) {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	stamp := func(i int) string { return db.FormatTime(start.Add(time.Duration(i) * time.Minute)) }
	tx, err := d.Write.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	n := 0
	for p := range plants {
		plant := fmt.Sprintf("p%03d", p)
		exec(`INSERT INTO plants (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, plant, plant, stamp(0), stamp(0))
		for k := range tasksPerPlant {
			task := fmt.Sprintf("%s-t%d", plant, k)
			ids = append(ids, task)
			anchor := []string{"from_completion", "fixed"}[rng.IntN(2)]
			exec(`INSERT INTO care_tasks (id, plant_id, kind, interval_days, anchor, start_on, seasonal, moisture_check, water_at, created_at, updated_at)
				VALUES (?, ?, 'water', ?, ?, '2023-01-01', ?, ?, ?, ?, ?)`,
				task, plant, 1+rng.IntN(14), anchor, rng.IntN(2), rng.IntN(2), 1+rng.IntN(9), stamp(0), stamp(0))
			day := 0
			for range rng.IntN(events + 1) {
				n++
				if everyday {
					day += 1 + rng.IntN(3)
				} else {
					day += rng.IntN(4) // several a day, sometimes
				}
				on := start.AddDate(0, 0, day)
				if rng.IntN(10) == 0 {
					on = on.AddDate(0, 0, -rng.IntN(30)) // backdated
				}
				kind := []string{"done", "done", "done", "skipped", "snoozed", "checked", "checked", "checked"}[rng.IntN(8)]
				var snooze, reading, voided any
				switch kind {
				case "snoozed":
					snooze = on.AddDate(0, 0, 1+rng.IntN(5)).Format(time.DateOnly)
				case "checked":
					reading = 1 + rng.IntN(10)
				case "done":
					if rng.IntN(3) == 0 {
						reading = 1 + rng.IntN(10)
					}
				}
				if rng.IntN(15) == 0 {
					voided = stamp(n)
				}
				// Shuffled creation stamps: insertion order mustn't follow the date.
				exec(`INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, reading, occurred_at, voided_at, created_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					fmt.Sprintf("e%07d", n), task, kind, on.Format(time.DateOnly), snooze, reading, stamp(n), voided, stamp(n+rng.IntN(3)))
			}
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return d, ids
}

// loadEvents keeps every event a schedule depends on: each task's dates come out the
// same as from its whole log.
func TestScheduleEventsGiveTheFullLogsDates(t *testing.T) {
	ctx := context.Background()
	for seed := range uint64(20) {
		d, ids := history(t, seed, 4, 3, 120, false)
		q := d.Reader()
		rows, err := q.ListActiveTasks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		byTask, err := loadEvents(ctx, q, rows)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			task, err := q.GetTask(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			all, err := q.ListTaskEvents(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			for _, month := range []time.Month{time.January, time.July} {
				cal := Calendar{Today: schedule.Date{Year: 2026, Month: month, Day: 1}, Hemisphere: schedule.North}
				want, err := Schedule(task, all, cal)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Schedule(task, byTask[id], cal)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("seed %d, task %s (soil check %d, %d of %d events kept):\n got %+v\nwant %+v",
						seed, id, task.MoistureCheck, len(byTask[id]), len(all), got, want)
				}
			}
		}
	}
}

// 50 plants × 3 tasks × 3 years of care, an event every two days or so: what Today
// loads stays a handful per task.
func TestScheduleEventsAreBoundedPerTask(t *testing.T) {
	d, ids := history(t, 1, 50, 3, 3*365/2, true)
	var total int
	if err := d.Read.QueryRow(`SELECT COUNT(*) FROM care_events`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	rows, err := d.Reader().ListActiveTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byTask, err := loadEvents(context.Background(), d.Reader(), rows)
	if err != nil {
		t.Fatal(err)
	}
	kept, most := 0, 0
	for _, evs := range byTask {
		kept += len(evs)
		most = max(most, len(evs))
	}
	t.Logf("%d tasks: %d of %d events kept, at most %d for a task", len(ids), kept, total, most)
	if most > 30 {
		t.Errorf("a task kept %d events; want a bounded handful", most)
	}
}

// A soil-check task last read the meter at a watering older than its fourth-latest one.
// Loaded without it, the soil-check events would start at day 10, and that old reading,
// kept as the latest, would end a made-up cycle at day 10: 9 days instead of 4 and 5.
// The median of the last three would be 9, not 5.
func TestScheduleEventsKeepSoilChecksBackToTheLatestReading(t *testing.T) {
	d, _ := history(t, 0, 0, 0, 0, false)
	ctx := context.Background()
	for _, q := range []string{
		`INSERT INTO plants (id, name, created_at, updated_at) VALUES ('p', 'p', '2023-01-01T00:00:00Z', '2023-01-01T00:00:00Z')`,
		`INSERT INTO care_tasks (id, plant_id, kind, interval_days, start_on, moisture_check, created_at, updated_at)
			VALUES ('t', 'p', 'water', 7, '2023-01-01', 1, '2023-01-01T00:00:00Z', '2023-01-01T00:00:00Z')`,
	} {
		if _, err := d.Write.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	for i, day := range []int{1, 5, 10, 12, 32, 32} {
		var reading any
		if day == 1 {
			reading = 5
		}
		on := time.Date(2023, 1, day, 0, 0, 0, 0, time.UTC)
		if _, err := d.Write.ExecContext(ctx, `INSERT INTO care_events (id, task_id, kind, local_date, reading, occurred_at, created_at)
			VALUES (?, 't', 'done', ?, ?, ?, ?)`, fmt.Sprint(i), on.Format(time.DateOnly), reading, db.FormatTime(on), db.FormatTime(on)); err != nil {
			t.Fatal(err)
		}
	}
	q := d.Reader()
	rows, err := q.ListActiveTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	events, err := loadEvents(ctx, q, rows)
	if err != nil {
		t.Fatal(err)
	}
	dates, err := Schedule(rows[0].CareTask, events["t"], Calendar{Today: schedule.Date{Year: 2023, Month: 2, Day: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !dates.HasDrying || dates.DryingDays != 5 {
		t.Errorf("drying days = %d (%v), want 5", dates.DryingDays, dates.HasDrying)
	}
}
