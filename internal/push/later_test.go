package push

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/schedule"
)

func TestWithLater(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 3, 1, h, m, 0, 0, chicago) }
	first := digestPayload{UserID: "u", Date: "2026-03-01"}
	again := digestPayload{UserID: "u", Date: "2026-03-01", Later: true}
	cases := []struct {
		name  string
		p     digestPayload
		later string
		now   time.Time
		offer bool
	}{
		{"before the later time", first, "17:30", at(8, 0), true},
		{"a minute before", first, "17:30", at(17, 29), true},
		{"at the later time", first, "17:30", at(17, 30), false},
		{"after it", first, "17:30", at(20, 0), false},
		{"the postponed reminder", again, "17:30", at(8, 0), false},
		{"an unreadable later time", first, "evening", at(8, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := withLater(Message{Title: "1 plant needs care"}, c.p, c.later, c.now, chicago)
			if offered := m.LaterTime != ""; offered != c.offer {
				t.Fatalf("offered = %v, want %v (%+v)", offered, c.offer, m)
			}
			if c.offer && (m.LaterTime != c.later || m.Date != "2026-03-01") {
				t.Errorf("message = %+v", m)
			}
			if !c.offer && m.Date != "" {
				t.Errorf("date set without the offer: %+v", m)
			}
		})
	}
}

func TestRemindLaterSendsAgain(t *testing.T) {
	f := newFixture(t, "America/Chicago", "08:00")
	at := func(h, m int) time.Time { return time.Date(2026, 3, 1, h, m, 0, 0, chicago) }
	march1, _ := schedule.Parse("2026-03-01")
	ctx := context.Background()

	if got := f.tick(at(8, 0)); got != 1 {
		t.Fatalf("pushes after the reminder = %d, want 1", got)
	}
	for range 2 {
		when, err := RemindLater(ctx, f.digest.DB, f.userID, march1, at(9, 15))
		if err != nil || !when.Equal(at(17, 30)) {
			t.Fatalf("RemindLater = %s, %v; want 17:30", when, err)
		}
	}
	if got := f.tick(at(17, 29)); got != 1 {
		t.Errorf("pushes before the later time = %d, want 1", got)
	}
	if got := f.tick(at(17, 30)); got != 2 {
		t.Errorf("pushes at the later time = %d, want 2", got)
	}
	if got := f.tick(at(18, 0)); got != 2 {
		t.Errorf("pushes after = %d, want 2 (sent once)", got)
	}
	jobs := f.jobs()
	if len(jobs) != 2 || jobs[1].Status != "done" || jobs[1].Note.Valid {
		t.Errorf("jobs = %+v", jobs)
	}

	if _, err := RemindLater(ctx, f.digest.DB, f.userID, march1, at(17, 30)); !errors.Is(err, ErrTooLate) {
		t.Errorf("at the later time: err = %v, want ErrTooLate", err)
	}
	if _, err := RemindLater(ctx, f.digest.DB, f.userID, march1, at(9, 0).AddDate(0, 0, 1)); !errors.Is(err, ErrNotToday) {
		t.Errorf("the next day: err = %v, want ErrNotToday", err)
	}
}

func TestRemindLaterSkipsWhenNothingIsDue(t *testing.T) {
	f := newFixture(t, "America/Chicago", "08:00")
	at := func(h, m int) time.Time { return time.Date(2026, 2, 15, h, m, 0, 0, chicago) }
	feb15, _ := schedule.Parse("2026-02-15") // Monstera's first watering is March 1
	if _, err := RemindLater(context.Background(), f.digest.DB, f.userID, feb15, at(9, 0)); err != nil {
		t.Fatal(err)
	}
	if got := f.tick(at(17, 30)); got != 0 {
		t.Errorf("pushes = %d, want 0", got)
	}
	jobs := f.jobs()
	last := jobs[len(jobs)-1]
	if last.Status != "done" || last.Note.String != "nothing due" {
		t.Errorf("later job = %+v", last)
	}
}
