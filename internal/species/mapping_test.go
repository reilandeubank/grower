package species

import (
	"strconv"
	"testing"
)

// text renders an optional value for test comparisons.
func text[T int | int64 | string](p *T) string {
	switch v := any(p).(type) {
	case *int:
		if v != nil {
			return strconv.Itoa(*v)
		}
	case *int64:
		if v != nil {
			return strconv.FormatInt(*v, 10)
		}
	case *string:
		if v != nil {
			return *v
		}
	}
	return "nil"
}

func ptr[T any](v T) *T { return &v }

func TestPerenualInterval(t *testing.T) {
	tests := []struct {
		watering, value, unit string
		want                  string
	}{
		{"Average", "7-10", "days", "9"}, // 8.5 rounds up
		{"Average", "5-7", "days", "6"},
		{"Average", `"7-10"`, "days", "9"},
		{"Frequent", "3", "days", "3"},
		{"Minimum", "1-2", "weeks", "11"}, // 10.5 days
		{"Minimum", "2", "week", "14"},
		{"Average", "4 - 6", "", "5"},
		{"Frequent", "", "", "3"},
		{"average", "", "", "7"},
		{"Minimum", "", "", "14"},
		{"None", "", "", "nil"},
		{"Upgrade Plans To Premium", "", "", "nil"},
		{"Average", "soon", "days", "7"}, // unreadable benchmark: fall back to the word
		{"Average", "1", "months", "7"},  // unknown unit
		{"Average", "0", "days", "7"},    // out of range
		{"Average", "400", "days", "7"},  // out of range
		{"", "10-14", "days", "12"},
	}
	for _, tt := range tests {
		if got := text(perenualInterval(tt.watering, tt.value, tt.unit)); got != tt.want {
			t.Errorf("perenualInterval(%q, %q, %q) = %s, want %s", tt.watering, tt.value, tt.unit, got, tt.want)
		}
	}
}

func TestPerenualLight(t *testing.T) {
	tests := []struct {
		sunlight []string
		want     string
	}{
		{[]string{"full sun", "part shade"}, "direct"},
		{[]string{"Part shade"}, "bright_indirect"},
		{[]string{"Part sun/part shade"}, "bright_indirect"},
		{[]string{"sun-part shade"}, "bright_indirect"},
		{[]string{"filtered shade"}, "medium"},
		{[]string{"deep shade", "filtered shade"}, "medium"},
		{[]string{"full shade"}, "low"},
		{[]string{"Deep shade"}, "low"},
		{[]string{"moonlight"}, "nil"},
		{nil, "nil"},
	}
	for _, tt := range tests {
		if got := text(perenualLight(tt.sunlight)); got != tt.want {
			t.Errorf("perenualLight(%q) = %s, want %s", tt.sunlight, got, tt.want)
		}
	}
}

func TestPlantbookLight(t *testing.T) {
	tests := []struct {
		lux  *int64
		want string
	}{
		{ptr[int64](80000), "direct"},
		{ptr[int64](40000), "direct"},
		{ptr[int64](39999), "bright_indirect"},
		{ptr[int64](15000), "bright_indirect"}, // Monstera deliciosa (live data, 2026-10-19)
		{ptr[int64](10000), "bright_indirect"},
		{ptr[int64](9999), "medium"},
		{ptr[int64](3000), "medium"},
		{ptr[int64](2999), "low"},
		{ptr[int64](0), "low"},
		{nil, "nil"},
	}
	for _, tt := range tests {
		if got := text(plantbookLight(tt.lux)); got != tt.want {
			t.Errorf("plantbookLight(%v) = %s, want %s", text(tt.lux), got, tt.want)
		}
	}
}

func TestPlantbookWaterAt(t *testing.T) {
	tests := []struct {
		moist *int64
		want  string
	}{
		{ptr[int64](15), "2"}, // 1.5 rounds up
		{ptr[int64](30), "3"},
		{ptr[int64](34), "3"},
		{ptr[int64](0), "1"},
		{ptr[int64](4), "1"},
		{ptr[int64](95), "9"},
		{ptr[int64](-10), "1"},
		{nil, "nil"},
	}
	for _, tt := range tests {
		if got := text(plantbookWaterAt(tt.moist)); got != tt.want {
			t.Errorf("plantbookWaterAt(%s) = %s, want %s", text(tt.moist), got, tt.want)
		}
	}
}
