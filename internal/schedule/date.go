package schedule

import (
	"fmt"
	"time"
)

// Date is a civil date with no time zone. The zero value is not a valid date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the date t falls on in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{y, m, d}
}

// Parse reads a YYYY-MM-DD date.
func Parse(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return DateOf(t), nil
}

// String formats d as YYYY-MM-DD.
func (d Date) String() string { return d.noon().Format(time.DateOnly) }

// AddDays returns d moved by n days (n may be negative).
func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Year, d.Month, d.Day+n, 12, 0, 0, 0, time.UTC))
}

// Sub returns the number of days from o to d.
func (d Date) Sub(o Date) int { return int(d.noon().Sub(o.noon()).Hours() / 24) }

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int { return d.noon().Compare(o.noon()) }

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is after o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// noon is d at midday UTC, where every day is 24 hours long.
func (d Date) noon() time.Time { return time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC) }

// Today is the date it is at now in loc. It is the only time zone-aware function here.
func Today(now time.Time, loc *time.Location) Date { return DateOf(now.In(loc)) }
