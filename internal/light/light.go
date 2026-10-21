// Package light derives how much light a room and a plant get (PLAN §5.6). It is pure:
// no clock, no DB. String values match the API and database.
package light

import (
	"slices"

	"github.com/reilandeubank/grower/internal/schedule"
)

// Level is a light level, ordered low < medium < bright_indirect < direct.
type Level string

const (
	Low            Level = "low"
	Medium         Level = "medium"
	BrightIndirect Level = "bright_indirect"
	Direct         Level = "direct"
)

var levels = []Level{Low, Medium, BrightIndirect, Direct}

func (l Level) rank() int { return slices.Index(levels, l) }

// Valid reports whether l is a known level.
func (l Level) Valid() bool { return l.rank() >= 0 }

// Less reports whether l is dimmer than m.
func (l Level) Less(m Level) bool { return l.rank() < m.rank() }

// Step moves n levels brighter (negative: dimmer), staying within low..direct.
func (l Level) Step(n int) Level {
	return levels[min(max(l.rank()+n, 0), len(levels)-1)]
}

// Direction is the way a window faces.
type Direction string

const (
	N  Direction = "n"
	NE Direction = "ne"
	E  Direction = "e"
	SE Direction = "se"
	S  Direction = "s"
	SW Direction = "sw"
	W  Direction = "w"
	NW Direction = "nw"
)

// directions is the compass order windows are listed in.
var directions = []Direction{N, NE, E, SE, S, SW, W, NW}

// Valid reports whether d is a known direction.
func (d Direction) Valid() bool { return slices.Contains(directions, d) }

// Window is a number of windows facing one direction.
type Window struct {
	Direction Direction
	Count     int
}

// SortWindows orders windows by compass direction, north first, clockwise.
func SortWindows(ws []Window) {
	slices.SortFunc(ws, func(a, b Window) int {
		return slices.Index(directions, a.Direction) - slices.Index(directions, b.Direction)
	})
}

// northern is what one window gives in the northern hemisphere. The southern hemisphere
// mirrors it north–south, since the sun is on the other side.
var northern = map[Direction]Level{
	S: Direct, E: BrightIndirect, W: BrightIndirect, SE: BrightIndirect, SW: BrightIndirect,
	N: Medium, NE: Medium, NW: Medium,
}

var mirror = map[Direction]Direction{N: S, NE: SE, E: E, SE: NE, S: N, SW: NW, W: W, NW: SW}

// Suggested estimates a room's light from its windows: the best window decides, and two
// or more windows in total raise a medium result to bright_indirect. ok is false
// without windows.
func Suggested(windows []Window, h schedule.Hemisphere) (Level, bool) {
	var best Level
	total := 0
	for _, w := range windows {
		if w.Count < 1 {
			continue
		}
		d := w.Direction
		if h == schedule.South {
			d = mirror[d]
		}
		if l, known := northern[d]; known && (best == "" || best.Less(l)) {
			best = l
		}
		total += w.Count
	}
	if best == Medium && total >= 2 {
		best = BrightIndirect
	}
	return best, best != ""
}

// Source says where an effective light level comes from.
type Source string

const (
	FromPlant     Source = "plant"
	FromLocation  Source = "location"
	FromWindows   Source = "windows"
	FromGrowLight Source = "grow_light"
)

// Light is a level and where it comes from. The zero value means unknown.
type Light struct {
	Level  Level
	Source Source
}

// Room is a location's light: its set level ("" when unset), else the suggestion from
// its windows.
func Room(set Level, windows []Window, h schedule.Hemisphere) (Light, bool) {
	if set != "" {
		return Light{set, FromLocation}, true
	}
	l, ok := Suggested(windows, h)
	if !ok {
		return Light{}, false
	}
	return Light{l, FromWindows}, true
}

// Distance is how far a plant stands from the window; "" when not recorded.
type Distance string

const (
	AtWindow   Distance = "at_window"
	Near       Distance = "near"
	AcrossRoom Distance = "across_room"
)

var dimming = map[Distance]int{AtWindow: 0, Near: 1, AcrossRoom: 2}

// Effective is the light a plant gets. Its own level ("" when unset) overrides
// everything. Otherwise the room's light (zero when unknown) is dimmed by distance, then
// a grow light (growHours 0 when none) raises it to at least bright_indirect from 10 h a
// day, or medium below that; the source becomes grow_light only if it raised the level.
// ok is false when nothing is known.
func Effective(plant Level, room Light, d Distance, growHours int) (Light, bool) {
	if plant != "" {
		return Light{plant, FromPlant}, true
	}
	l := room
	if l.Level != "" {
		l.Level = l.Level.Step(-dimming[d])
	}
	if growHours > 0 {
		floor := Medium
		if growHours >= 10 {
			floor = BrightIndirect
		}
		if l.Level == "" || l.Level.Less(floor) {
			l = Light{floor, FromGrowLight}
		}
	}
	return l, l.Level != ""
}
