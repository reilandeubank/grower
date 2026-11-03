package push

import (
	"database/sql"
	"testing"

	"github.com/reilandeubank/grower/internal/care"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

func item(plantID, plant, kind, label string) care.Item {
	return care.Item{Plant: dbgen.Plant{ID: plantID}, PlantName: plant, Task: dbgen.CareTask{Kind: kind, Label: label}}
}

func TestDigestMessage(t *testing.T) {
	monstera := item("1", "Monstera", "water", "")
	fern := item("2", "Fern", "mist", "")
	pothos := item("3", "Pothos", "custom", "Prune")
	monsteraFeed := item("1", "Monstera", "fertilize", "")
	ficusCheck := item("4", "Ficus", "water", "")
	ficusCheck.Task.MoistureCheck = 1
	basilCheckIn := item("5", "Basil", "check_in", "")
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
		{"soil check", nil, []care.Item{ficusCheck, fern}, "2 plants need care", "Check soil for Ficus and Mist Fern"},
		{"check-in", nil, []care.Item{basilCheckIn}, "1 plant needs care", "Check in on Basil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := DigestMessage(care.Agenda{Overdue: tt.overdue, Due: tt.due}, nil)
			if !ok || m.Title != tt.title || m.Body != tt.body || m.URL != "/" || m.Tag != "digest" {
				t.Errorf("got %+v, %v; want %q / %q", m, ok, tt.title, tt.body)
			}
		})
	}
	if _, ok := DigestMessage(care.Agenda{Upcoming: []care.Item{monstera}}, nil); ok {
		t.Error("upcoming tasks alone made a reminder")
	}
}

func TestDigestMessageSharedNames(t *testing.T) {
	inRoom := func(it care.Item, room string) care.Item {
		if room != "" {
			it.Plant.LocationID = sql.NullString{String: room, Valid: true}
		}
		return it
	}
	rooms := map[string]string{"office": "Office", "living": "Living room"}
	office := inRoom(item("1", "Pothos", "water", ""), "office")
	living := inRoom(item("2", "Pothos", "water", ""), "living")
	nowhere := item("3", "pothos", "water", "")
	fern := inRoom(item("4", "Fern", "mist", ""), "office")
	tests := []struct {
		name   string
		agenda care.Agenda
		body   string
	}{
		{"same name, two rooms", care.Agenda{Due: []care.Item{office, living, fern}}, "Water Pothos (Office), Water Pothos (Living room) and Mist Fern"},
		{"twin only upcoming", care.Agenda{Due: []care.Item{office}, Upcoming: []care.Item{living}}, "Water Pothos (Office)"},
		{"no room to tell by", care.Agenda{Due: []care.Item{office, nowhere}}, "Water Pothos (Office) and Water pothos"},
		{"one plant, two tasks", care.Agenda{Due: []care.Item{office, inRoom(item("1", "Pothos", "mist", ""), "office")}}, "Water Pothos and Mist Pothos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if m, _ := DigestMessage(tt.agenda, rooms); m.Body != tt.body {
				t.Errorf("body = %q, want %q", m.Body, tt.body)
			}
		})
	}
}

func TestParseClock(t *testing.T) {
	for s, ok := range map[string]bool{"08:00": true, "00:00": true, "23:59": true, "8:00": false, "24:00": false, "08:60": false, "": false, "08:00:00": false, "0800": false} {
		if _, err := ParseClock(s); (err == nil) != ok {
			t.Errorf("ParseClock(%q) err = %v", s, err)
		}
	}
}
