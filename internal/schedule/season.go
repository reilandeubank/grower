package schedule

import "time"

// Hemisphere decides which months are winter. Values match the API and database; the
// zero value counts as North.
type Hemisphere string

const (
	North Hemisphere = "north"
	South Hemisphere = "south"
)

// seasonalPercent is the built-in curve per task kind for the northern hemisphere, in
// percent of the interval by month (January first): plants drink and feed less in winter
// and a little more in high summer. Kinds not listed keep their interval all year.
// Percentages keep the arithmetic exact.
var seasonalPercent = map[string][12]int{
	"water":     {150, 140, 120, 100, 100, 90, 85, 90, 100, 120, 140, 150},
	"mist":      {150, 140, 120, 100, 100, 90, 85, 90, 100, 120, 140, 150},
	"fertilize": {200, 200, 125, 100, 100, 100, 100, 100, 125, 150, 200, 200},
}

// percent is a kind's curve value for a month. The southern hemisphere's seasons are six
// months off the northern ones.
func percent(kind string, m time.Month, h Hemisphere) int {
	curve, ok := seasonalPercent[kind]
	if !ok {
		return 100
	}
	i := int(m) - 1
	if h == South {
		i = (i + 6) % 12
	}
	return curve[i]
}

// interval is the interval counted from a date: max(1, round(IntervalDays × factor of
// on's month)) for a seasonal task counted from completion (PLAN §5.2 rule 8), else
// IntervalDays. A fixed anchor keeps its calendar, so it is never adjusted.
func (t Task) interval(on Date, h Hemisphere) int {
	if !t.Seasonal || t.Anchor == Fixed {
		return t.IntervalDays
	}
	return max(1, (t.IntervalDays*percent(t.Kind, on.Month, h)+50)/100)
}

// CurrentInterval is the interval that would apply if the task were done today: the API's
// current_interval_days. The next due date itself uses the month of the last completion,
// so the two differ only briefly after a month boundary.
func CurrentInterval(t Task, today Date, h Hemisphere) int { return t.interval(today, h) }
