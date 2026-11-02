package care

import (
	"database/sql"
	"testing"

	"github.com/reilandeubank/grower/internal/db/dbgen"
)

func TestNamesName(t *testing.T) {
	names := Names{"snake": "Snake plant", "ficus": "Ficus lyrata"}
	linked := func(id string) sql.NullString { return sql.NullString{String: id, Valid: true} }
	tests := []struct {
		name  string
		plant dbgen.Plant
		want  string
	}{
		{"own name wins", dbgen.Plant{Name: "Sansa", Species: "Dracaena trifasciata", SpeciesID: linked("snake")}, "Sansa"},
		{"linked species' label", dbgen.Plant{Species: "Dracaena trifasciata", SpeciesID: linked("snake")}, "Snake plant"},
		{"label is the scientific name without a common one", dbgen.Plant{Species: "Ficus lyrata", SpeciesID: linked("ficus")}, "Ficus lyrata"},
		{"free-text species", dbgen.Plant{Species: "monstera"}, "monstera"},
		{"unknown link falls back to the text", dbgen.Plant{Species: "Pothos", SpeciesID: linked("gone")}, "Pothos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names.Name(tt.plant); got != tt.want {
				t.Errorf("Name = %q, want %q", got, tt.want)
			}
		})
	}
}
