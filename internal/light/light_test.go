package light

import (
	"testing"

	"github.com/reilandeubank/grower/internal/schedule"
)

func TestStep(t *testing.T) {
	tests := []struct {
		l    Level
		n    int
		want Level
	}{
		{Direct, 0, Direct},
		{Direct, -1, BrightIndirect},
		{Direct, -2, Medium},
		{BrightIndirect, -2, Low},
		{Medium, -2, Low}, // never below low
		{Low, -1, Low},
		{Low, 1, Medium},
		{BrightIndirect, 5, Direct},
	}
	for _, tt := range tests {
		if got := tt.l.Step(tt.n); got != tt.want {
			t.Errorf("%s.Step(%d) = %s, want %s", tt.l, tt.n, got, tt.want)
		}
	}
	if !Low.Less(Medium) || !Medium.Less(BrightIndirect) || !BrightIndirect.Less(Direct) || Direct.Less(Low) || Low.Less(Low) {
		t.Error("levels are not ordered low < medium < bright_indirect < direct")
	}
	if Level("dim").Valid() || Level("").Valid() || !Direct.Valid() {
		t.Error("Valid is wrong")
	}
}

func TestSuggested(t *testing.T) {
	one := func(d Direction) []Window { return []Window{{d, 1}} }
	n, s := schedule.North, schedule.South
	tests := []struct {
		name    string
		windows []Window
		h       schedule.Hemisphere
		want    Level // "" means no suggestion
	}{
		{"no windows", nil, n, ""},
		{"zero count is no window", []Window{{S, 0}}, n, ""},
		// One window each way, northern hemisphere.
		{"north: n", one(N), n, Medium},
		{"north: ne", one(NE), n, Medium},
		{"north: e", one(E), n, BrightIndirect},
		{"north: se", one(SE), n, BrightIndirect},
		{"north: s", one(S), n, Direct},
		{"north: sw", one(SW), n, BrightIndirect},
		{"north: w", one(W), n, BrightIndirect},
		{"north: nw", one(NW), n, Medium},
		{"zero hemisphere is north", one(S), "", Direct},
		// Southern hemisphere mirrors it.
		{"south: n", one(N), s, Direct},
		{"south: ne", one(NE), s, BrightIndirect},
		{"south: e", one(E), s, BrightIndirect},
		{"south: se", one(SE), s, Medium},
		{"south: s", one(S), s, Medium},
		{"south: sw", one(SW), s, Medium},
		{"south: w", one(W), s, BrightIndirect},
		{"south: nw", one(NW), s, BrightIndirect},
		// The best window decides.
		{"north: n and e", []Window{{N, 1}, {E, 1}}, n, BrightIndirect},
		{"north: n, w and s", []Window{{N, 1}, {W, 1}, {S, 1}}, n, Direct},
		// Two or more windows raise medium only.
		{"north: two n windows", []Window{{N, 2}}, n, BrightIndirect},
		{"north: n and nw", []Window{{N, 1}, {NW, 1}}, n, BrightIndirect},
		{"south: s and se", []Window{{S, 1}, {SE, 1}}, s, BrightIndirect},
		{"north: two e windows stay", []Window{{E, 2}}, n, BrightIndirect},
		{"north: nine s windows stay", []Window{{S, 9}}, n, Direct},
	}
	for _, tt := range tests {
		got, ok := Suggested(tt.windows, tt.h)
		if ok != (tt.want != "") || got != tt.want {
			t.Errorf("%s: Suggested = %q, %v; want %q", tt.name, got, ok, tt.want)
		}
	}
}

func TestRoom(t *testing.T) {
	tests := []struct {
		name    string
		set     Level
		windows []Window
		h       schedule.Hemisphere
		want    Light
		ok      bool
	}{
		{"nothing", "", nil, schedule.North, Light{}, false},
		{"set wins over windows", Low, []Window{{S, 1}}, schedule.North, Light{Low, FromLocation}, true},
		{"set without windows", Direct, nil, schedule.North, Light{Direct, FromLocation}, true},
		{"windows north", "", []Window{{S, 1}}, schedule.North, Light{Direct, FromWindows}, true},
		{"windows south", "", []Window{{S, 1}}, schedule.South, Light{Medium, FromWindows}, true},
	}
	for _, tt := range tests {
		got, ok := Room(tt.set, tt.windows, tt.h)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: Room = %+v, %v; want %+v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestEffective(t *testing.T) {
	windows := Light{Direct, FromWindows}
	set := Light{BrightIndirect, FromLocation}
	tests := []struct {
		name  string
		plant Level
		room  Light
		d     Distance
		hours int
		want  Light
		ok    bool
	}{
		{"nothing known", "", Light{}, "", 0, Light{}, false},
		{"distance alone is nothing", "", Light{}, AcrossRoom, 0, Light{}, false},
		{"plant light", Low, windows, AtWindow, 0, Light{Low, FromPlant}, true},
		{"plant light overrides grow light", Low, Light{}, "", 12, Light{Low, FromPlant}, true},
		{"room, no distance", "", windows, "", 0, windows, true},
		{"room at window", "", set, AtWindow, 0, set, true},
		{"room near", "", windows, Near, 0, Light{BrightIndirect, FromWindows}, true},
		{"room across", "", windows, AcrossRoom, 0, Light{Medium, FromWindows}, true},
		{"across set location", "", set, AcrossRoom, 0, Light{Low, FromLocation}, true},
		{"never below low", "", Light{Medium, FromLocation}, AcrossRoom, 0, Light{Low, FromLocation}, true},
		{"grow light alone, 10 h", "", Light{}, "", 10, Light{BrightIndirect, FromGrowLight}, true},
		{"grow light alone, 9 h", "", Light{}, "", 9, Light{Medium, FromGrowLight}, true},
		{"grow light alone, 1 h", "", Light{}, "", 1, Light{Medium, FromGrowLight}, true},
		{"grow light raises dim room, 24 h", "", Light{Low, FromLocation}, "", 24, Light{BrightIndirect, FromGrowLight}, true},
		{"grow light raises after distance", "", windows, AcrossRoom, 10, Light{BrightIndirect, FromGrowLight}, true},
		{"9 h raises low to medium", "", Light{Medium, FromWindows}, Near, 9, Light{Medium, FromGrowLight}, true},
		{"9 h doesn't raise medium", "", Light{Medium, FromWindows}, AtWindow, 9, Light{Medium, FromWindows}, true},
		{"10 h doesn't raise bright", "", set, "", 10, set, true},
		{"10 h doesn't lower direct", "", windows, "", 10, windows, true},
	}
	for _, tt := range tests {
		got, ok := Effective(tt.plant, tt.room, tt.d, tt.hours)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: Effective = %+v, %v; want %+v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSortWindows(t *testing.T) {
	ws := []Window{{NW, 1}, {S, 2}, {N, 1}, {E, 3}}
	SortWindows(ws)
	want := []Window{{N, 1}, {E, 3}, {S, 2}, {NW, 1}}
	for i := range want {
		if ws[i] != want[i] {
			t.Fatalf("SortWindows = %v, want %v", ws, want)
		}
	}
}
