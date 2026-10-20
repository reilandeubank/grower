package species

import (
	"math"
	"strconv"
	"strings"
)

// Light levels, matching the API's LightLevel.
const (
	LightLow            = "low"
	LightMedium         = "medium"
	LightBrightIndirect = "bright_indirect"
	LightDirect         = "direct"
)

// perenualInterval maps Perenual's watering fields to days between waterings. The
// benchmark ("7-10" days, "1" week) wins; without a usable one, the watering word
// gives a rough default. "None" (and anything unknown) means no interval.
func perenualInterval(watering, benchmark, unit string) *int {
	if days, ok := benchmarkDays(benchmark, unit); ok {
		return &days
	}
	var days int
	switch strings.ToLower(strings.TrimSpace(watering)) {
	case "frequent":
		days = 3
	case "average":
		days = 7
	case "minimum":
		days = 14
	default:
		return nil
	}
	return &days
}

// benchmarkDays reads "a-b" (the rounded midpoint) or "a" in days or weeks. Perenual
// has sent the value both bare and wrapped in extra quotes ("\"7-10\""), so quotes go.
func benchmarkDays(value, unit string) (int, bool) {
	value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"`))
	if value == "" {
		return 0, false
	}
	lo, hi, isRange := strings.Cut(value, "-")
	a, err := strconv.ParseFloat(strings.TrimSpace(lo), 64)
	if err != nil {
		return 0, false
	}
	b := a
	if isRange {
		if b, err = strconv.ParseFloat(strings.TrimSpace(hi), 64); err != nil {
			return 0, false
		}
	}
	per := 1.0
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "day", "days":
	case "week", "weeks":
		per = 7
	default:
		return 0, false
	}
	days := int(math.Round((a + b) / 2 * per))
	if days < 1 || days > 365 {
		return 0, false
	}
	return days, true
}

// perenualLight maps Perenual's sunlight list to the brightest light level listed.
func perenualLight(sunlight []string) *string {
	rank := map[string]int{LightLow: 1, LightMedium: 2, LightBrightIndirect: 3, LightDirect: 4}
	best := ""
	for _, s := range sunlight {
		if l := sunlightLevel(s); rank[l] > rank[best] {
			best = l
		}
	}
	if best == "" {
		return nil
	}
	return &best
}

func sunlightLevel(s string) string {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "full sun"):
		return LightDirect
	case strings.Contains(s, "part sun"), strings.Contains(s, "part shade"): // incl. "sun-part shade"
		return LightBrightIndirect
	case strings.Contains(s, "filtered shade"):
		return LightMedium
	case strings.Contains(s, "full shade"), strings.Contains(s, "deep shade"):
		return LightLow
	}
	return ""
}

// plantbookLight maps the most light a plant thrives in (max_light_lux) to a light level.
// The minimum is only what it tolerates: Monstera deliciosa is 800–15000 lux, and its
// minimum would call it a low-light plant, while it wants bright, indirect light.
func plantbookLight(maxLux *int64) *string {
	if maxLux == nil {
		return nil
	}
	l := LightLow
	switch {
	case *maxLux >= 40000:
		l = LightDirect
	case *maxLux >= 10000:
		l = LightBrightIndirect
	case *maxLux >= 3000:
		l = LightMedium
	}
	return &l
}

// plantbookWaterAt maps the driest soil a plant tolerates (percent) to the moisture
// meter level to water at: 10 % per meter point, within the 1–9 water_at range.
func plantbookWaterAt(minSoilMoist *int64) *int {
	if minSoilMoist == nil {
		return nil
	}
	v := min(max(int(math.Round(float64(*minSoilMoist)/10)), 1), 9)
	return &v
}
