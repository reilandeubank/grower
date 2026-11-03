package care

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/reilandeubank/grower/internal/db/dbgen"
)

// Names maps species IDs to what a linked plant without a name is called: the
// species' common name, else its scientific name.
type Names map[string]string

// LoadNames reads every species' label, for Name.
func LoadNames(ctx context.Context, q *dbgen.Queries) (Names, error) {
	rows, err := q.ListSpeciesLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list species labels: %w", err)
	}
	n := make(Names, len(rows))
	for _, r := range rows {
		n[r.ID] = r.Label
	}
	return n, nil
}

// Name is what the app calls p: its own name, else its linked species' label ("Snake
// plant"), else the free-text species. A plant always has one of these (the API
// requires a name or a species).
func (n Names) Name(p dbgen.Plant) string {
	if p.Name != "" {
		return p.Name
	}
	if label := n[p.SpeciesID.String]; p.SpeciesID.Valid && label != "" {
		return label
	}
	return p.Species
}

// CompareNames orders plant names case-insensitively, for lists sorted by Name.
func CompareNames(a, b string) int {
	return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
}
