package db

import (
	"context"
	"strings"
	"testing"
)

// TestMoistureMigrationRoundTrip: 0008 rebuilds care_events, so existing history must
// survive up, down and up again, and the new constraints must hold.
func TestMoistureMigrationRoundTrip(t *testing.T) {
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
	exec := func(query string) error {
		t.Helper()
		_, err := d.Write.ExecContext(ctx, query)
		return err
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const stamp = "2026-05-01T00:00:00Z"
	event := func(id, kind, snooze, reading string) string {
		return `INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, reading, occurred_at, created_at)
			VALUES ('` + id + `', 't1', '` + kind + `', '2026-05-01', ` + snooze + `, ` + reading + `, '` + stamp + `', '` + stamp + `')`
	}
	// history lists every event as id:kind:snooze_until:voided.
	history := func() string {
		t.Helper()
		rows, err := d.Write.QueryContext(ctx, `SELECT id, kind, COALESCE(snooze_until, '-'), voided_at IS NOT NULL, note FROM care_events ORDER BY id`)
		must(err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, kind, snooze, note string
			var voided bool
			must(rows.Scan(&id, &kind, &snooze, &voided, &note))
			out = append(out, strings.Join([]string{id, kind, snooze, map[bool]string{true: "voided", false: "live"}[voided], note}, ":"))
		}
		must(rows.Err())
		return strings.Join(out, " ")
	}

	_, err = p.UpTo(ctx, 7)
	must(err)
	must(exec(`INSERT INTO plants (id, name, created_at, updated_at) VALUES ('p1', 'Fern', '` + stamp + `', '` + stamp + `')`))
	must(exec(`INSERT INTO care_tasks (id, plant_id, kind, interval_days, start_on, created_at, updated_at)
		VALUES ('t1', 'p1', 'water', 7, '2026-05-01', '` + stamp + `', '` + stamp + `')`))
	must(exec(`INSERT INTO care_events (id, task_id, kind, local_date, snooze_until, note, occurred_at, voided_at, created_at) VALUES
		('e1', 't1', 'done', '2026-05-01', NULL, 'Soaked', '` + stamp + `', NULL, '` + stamp + `'),
		('e2', 't1', 'skipped', '2026-05-02', NULL, '', '` + stamp + `', '` + stamp + `', '` + stamp + `'),
		('e3', 't1', 'snoozed', '2026-05-03', '2026-05-05', '', '` + stamp + `', NULL, '` + stamp + `')`))
	const want = "e1:done:-:live:Soaked e2:skipped:-:voided: e3:snoozed:2026-05-05:live:"
	if got := history(); got != want {
		t.Fatalf("before = %s", got)
	}

	_, err = p.UpTo(ctx, 8)
	must(err)
	if got := history(); got != want {
		t.Errorf("after up = %s", got)
	}
	var check, waterAt int
	must(d.Write.QueryRowContext(ctx, `SELECT moisture_check, water_at FROM care_tasks WHERE id = 't1'`).Scan(&check, &waterAt))
	if check != 0 || waterAt != 3 {
		t.Errorf("task defaults: moisture_check %d, water_at %d", check, waterAt)
	}
	for _, tc := range []struct {
		name, query string
		ok          bool
	}{
		{"check with a reading", event("n1", "checked", "NULL", "6"), true},
		{"done with a reading", event("n2", "done", "NULL", "2"), true},
		{"done without one", event("n3", "done", "NULL", "NULL"), true},
		{"skip without one", event("n4", "skipped", "NULL", "NULL"), true},
		{"check without a reading", event("x1", "checked", "NULL", "NULL"), false},
		{"reading on a snooze", event("x2", "snoozed", "'2026-05-09'", "5"), false},
		{"reading on a skip", event("x3", "skipped", "NULL", "5"), false},
		{"reading 0", event("x4", "checked", "NULL", "0"), false},
		{"reading 11", event("x5", "done", "NULL", "11"), false},
		{"water_at 10", `UPDATE care_tasks SET water_at = 10`, false},
		{"moisture_check 2", `UPDATE care_tasks SET moisture_check = 2`, false},
	} {
		if err := exec(tc.query); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.name, err)
		}
	}

	_, err = p.DownTo(ctx, 7)
	must(err)
	// Checks can't exist before 0008; everything else stays, readings dropped.
	if got := history(); got != want+" n2:done:-:live: n3:done:-:live: n4:skipped:-:live:" {
		t.Errorf("after down = %s", got)
	}
	if err := exec(event("x6", "checked", "NULL", "NULL")); err == nil {
		t.Error("0007 schema has a reading column")
	}

	_, err = p.UpTo(ctx, 8)
	must(err)
	if got := history(); got != want+" n2:done:-:live: n3:done:-:live: n4:skipped:-:live:" {
		t.Errorf("after up again = %s", got)
	}
	if err := exec(event("x7", "checked", "NULL", "NULL")); err == nil {
		t.Error("check without a reading accepted after the round trip")
	}
}
