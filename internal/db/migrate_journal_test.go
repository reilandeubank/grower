package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// TestJournalMigrationRoundTrip: 0011 rebuilds care_tasks, which care_events references
// with ON DELETE CASCADE. Run with foreign keys on, the rebuild would delete every event,
// so this checks that history survives up, down and up again byte for byte, and that
// foreign keys are enforced again afterwards on the writer connection.
func TestJournalMigrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := d.provider()
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	exec := func(query string) error {
		_, err := d.Write.ExecContext(ctx, query)
		return err
	}

	_, err = p.UpTo(ctx, 10)
	must(err)
	const stamp = "'2026-05-01T00:00:00Z'"
	must(exec(`INSERT INTO plants (id, name, created_at, updated_at) VALUES
		('p1', 'Monstera', ` + stamp + `, ` + stamp + `), ('p2', 'Basil', ` + stamp + `, ` + stamp + `)`))
	must(exec(`INSERT INTO photos (id, plant_id, taken_at, mime, width, height, bytes, sha256, created_at, updated_at) VALUES
		('ph1', 'p1', ` + stamp + `, 'image/jpeg', 10, 10, 100, 'aa', ` + stamp + `, ` + stamp + `),
		('ph2', 'p2', ` + stamp + `, 'image/png', 20, 10, 200, 'bb', ` + stamp + `, ` + stamp + `)`))
	must(exec(`UPDATE plants SET cover_photo_id = 'ph1' WHERE id = 'p1'`))
	must(exec(`INSERT INTO care_tasks (id, plant_id, kind, label, interval_days, anchor, start_on, seasonal, moisture_check, water_at, created_at, updated_at) VALUES
		('t-water', 'p1', 'water', '', 7, 'from_completion', '2026-04-01', 1, 0, 3, ` + stamp + `, ` + stamp + `),
		('t-soil', 'p2', 'water', '', 3, 'from_completion', '2026-04-02', 0, 1, 4, ` + stamp + `, ` + stamp + `),
		('t-fert', 'p1', 'fertilize', '', 30, 'fixed', '2026-04-03', 1, 0, 3, ` + stamp + `, ` + stamp + `),
		('t-mist', 'p1', 'mist', '', 2, 'from_completion', '2026-04-04', 0, 0, 3, ` + stamp + `, ` + stamp + `),
		('t-repot', 'p2', 'repot', '', 365, 'fixed', '2026-04-05', 0, 0, 3, ` + stamp + `, ` + stamp + `),
		('t-rotate', 'p2', 'rotate', '', 14, 'from_completion', '2026-04-06', 0, 0, 3, ` + stamp + `, ` + stamp + `),
		('t-custom', 'p1', 'custom', 'Wipe leaves', 21, 'fixed', '2026-04-07', 0, 0, 3, ` + stamp + `, ` + stamp + `)`))
	// Every event kind, readings, notes and voided rows, spread over all tasks.
	kinds := []string{"done", "skipped", "snoozed", "checked"}
	tasks := []string{"t-water", "t-soil", "t-fert", "t-mist", "t-repot", "t-rotate", "t-custom"}
	var values []string
	for i := range 60 {
		kind := kinds[i%len(kinds)]
		snooze, reading, voided := "NULL", "NULL", "NULL"
		switch kind {
		case "snoozed":
			snooze = "'2026-05-09'"
		case "checked":
			reading = fmt.Sprint(i%10 + 1)
		case "done":
			if i%8 == 0 {
				reading = "2"
			}
		}
		if i%5 == 0 {
			voided = stamp
		}
		values = append(values, fmt.Sprintf("('e%02d', '%s', '%s', '2026-05-%02d', %s, %s, 'note %d', %s, %s, %s)",
			i, tasks[i%len(tasks)], kind, i%28+1, snooze, reading, i, stamp, voided, stamp))
	}
	must(exec(`INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, reading, note, occurred_at, voided_at, created_at) VALUES ` +
		strings.Join(values, ",\n")))

	tables := []string{"plants", "photos", "care_tasks", "care_events"}
	before := dumpTables(t, d.Write, tables)
	if n := strings.Count(before["care_events"], "\n"); n != 60 {
		t.Fatalf("seeded %d events", n)
	}

	// checkSchema asserts the data is unchanged and that foreign keys hold and are enforced.
	checkSchema := func(stage string) {
		t.Helper()
		after := dumpTables(t, d.Write, tables)
		for _, table := range tables {
			if after[table] != before[table] {
				t.Errorf("%s: %s changed:\nbefore:\n%s\nafter:\n%s", stage, table, before[table], after[table])
			}
		}
		var fk int
		must(d.Write.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk))
		if fk != 1 {
			t.Errorf("%s: foreign_keys = %d on the writer connection", stage, fk)
		}
		if v := dumpQuery(t, d.Write, `PRAGMA foreign_key_check`); v != "" {
			t.Errorf("%s: foreign_key_check:\n%s", stage, v)
		}
		if v := dumpQuery(t, d.Write, `PRAGMA integrity_check`); v != "ok\n" {
			t.Errorf("%s: integrity_check: %s", stage, v)
		}
		if v := dumpQuery(t, d.Write, `SELECT "table", "from", "to", on_delete FROM pragma_foreign_key_list('care_events')`); v != "care_tasks task_id id CASCADE\n" {
			t.Errorf("%s: care_events foreign key: %s", stage, v)
		}
		if err := exec(`INSERT INTO care_events (id, task_id, kind, local_date, occurred_at, created_at)
			VALUES ('orphan', 'missing', 'done', '2026-05-01', ` + stamp + `, ` + stamp + `)`); !IsForeignKeyViolation(err) {
			t.Errorf("%s: event for a missing task: err = %v", stage, err)
		}
		// Cascades are checked in a transaction that is rolled back, keeping the data for later stages.
		tx, err := d.Write.BeginTx(ctx, nil)
		must(err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(ctx, `DELETE FROM care_tasks WHERE id = 't-water'`)
		must(err)
		var left int
		must(tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM care_events WHERE task_id = 't-water'`).Scan(&left))
		if left != 0 {
			t.Errorf("%s: deleting a task left %d events", stage, left)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM plants WHERE id = 'p2'`)
		must(err)
		must(tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM care_tasks WHERE plant_id = 'p2'`).Scan(&left))
		if left != 0 {
			t.Errorf("%s: deleting a plant left %d tasks", stage, left)
		}
	}
	checkIn := `INSERT INTO care_tasks (id, plant_id, kind, interval_days, start_on, created_at, updated_at)
		VALUES ('t-check', 'p1', 'check_in', 30, '2026-05-01', ` + stamp + `, ` + stamp + `)`

	_, err = p.UpTo(ctx, 11)
	must(err)
	checkSchema("up")
	must(exec(checkIn))
	must(exec(`INSERT INTO care_events (id, task_id, kind, local_date, occurred_at, created_at)
		VALUES ('e-check', 't-check', 'done', '2026-05-02', ` + stamp + `, ` + stamp + `)`))
	must(exec(`INSERT INTO plant_measurements (id, plant_id, measured_on, height_in, photo_id, created_at, updated_at)
		VALUES ('m1', 'p1', '2026-05-02', 12.5, 'ph1', ` + stamp + `, ` + stamp + `)`))
	if err := exec(`INSERT INTO plant_measurements (id, plant_id, measured_on, created_at, updated_at)
		VALUES ('m2', 'p1', '2026-05-02', ` + stamp + `, ` + stamp + `)`); err == nil {
		t.Error("empty measurement accepted")
	}

	// Down drops check-ins (and their events) and measurements; everything else stays.
	_, err = p.DownTo(ctx, 10)
	must(err)
	checkSchema("down")
	if err := exec(checkIn); err == nil {
		t.Error("check_in accepted after down")
	}

	_, err = p.UpTo(ctx, 11)
	must(err)
	checkSchema("up again")
	must(exec(checkIn))
}

// dumpTables renders every row of each table, in id order, one line per row.
func dumpTables(t *testing.T, db *sql.DB, tables []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(tables))
	for _, table := range tables {
		out[table] = dumpQuery(t, db, `SELECT * FROM `+table+` ORDER BY id`)
	}
	return out
}

func dumpQuery(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for i, v := range vals {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%v", v)
		}
		b.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
