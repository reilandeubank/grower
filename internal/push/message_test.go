package push

import (
	"testing"

	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

func item(plantID, plant, kind, label string) care.Item {
	return care.Item{Plant: dbgen.Plant{ID: plantID, Name: plant}, Task: dbgen.CareTask{Kind: kind, Label: label}}
}

func TestDigestMessage(t *testing.T) {
	monstera := item("1", "Monstera", "water", "")
	fern := item("2", "Fern", "mist", "")
	pothos := item("3", "Pothos", "custom", "Prune")
	monsteraFeed := item("1", "Monstera", "fertilize", "")
	tests := []struct {
		name         string
		overdue, due []care.Item
		title, body  string
	}{
		{"one", nil, []care.Item{monstera}, "1 plant needs care", "Water Monstera"},
		{"two", nil, []care.Item{monstera, fern}, "2 plants need care", "Water Monstera and Mist Fern"},
		{"overdue first", []care.Item{fern}, []care.Item{monstera}, "2 plants need care", "Mist Fern and Water Monstera"},
		{"one plant, two tasks", nil, []care.Item{monstera, monsteraFeed}, "1 plant needs care", "Water Monstera and Fertilize Monstera"},
		{"three", []care.Item{pothos}, []care.Item{monstera, fern}, "3 plants need care", "Prune for Pothos, Water Monstera and Mist Fern"},
		{"more", []care.Item{monstera, fern}, []care.Item{pothos, monsteraFeed}, "3 plants need care", "Water Monstera, Mist Fern, Prune for Pothos and 1 more"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := DigestMessage(care.Agenda{Overdue: tt.overdue, Due: tt.due})
			if !ok || m.Title != tt.title || m.Body != tt.body || m.URL != "/" || m.Tag != "digest" {
				t.Errorf("got %+v, %v; want %q / %q", m, ok, tt.title, tt.body)
			}
		})
	}
	if _, ok := DigestMessage(care.Agenda{Upcoming: []care.Item{monstera}}); ok {
		t.Error("upcoming tasks alone made a reminder")
	}
}

func TestParseClock(t *testing.T) {
	for s, ok := range map[string]bool{"08:00": true, "00:00": true, "23:59": true, "8:00": false, "24:00": false, "08:60": false, "": false, "08:00:00": false, "0800": false} {
		if _, err := ParseClock(s); (err == nil) != ok {
			t.Errorf("ParseClock(%q) err = %v", s, err)
		}
	}
}
