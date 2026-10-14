package server

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

func TestPlantsCRUD(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	loc := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Shelf"}`), http.StatusCreated)

	created := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants",
		`{"name":" Monty ","species":"Monstera deliciosa","location_id":"`+loc.Id+`","acquired_on":"2026-04-12","notes":"Repotted"}`),
		http.StatusCreated)
	if created.Id == "" || created.Name != "Monty" || created.Species != "Monstera deliciosa" || created.LocationId == nil ||
		*created.LocationId != loc.Id || created.AcquiredOn == nil || created.AcquiredOn.String() != "2026-04-12" ||
		created.Notes != "Repotted" || created.Archived || !created.CreatedAt.Equal(e.clock.t) {
		t.Errorf("created = %+v", created)
	}
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"aloe"}`), http.StatusCreated)

	path := "/api/v1/plants/" + created.Id
	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, path, ""), http.StatusOK); !reflect.DeepEqual(got, created) {
		t.Errorf("get = %+v, want %+v", got, created)
	}

	list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[0].Name != "aloe" || list.Items[1].Id != created.Id {
		t.Errorf("list not ordered by name ignoring case: %+v", list.Items)
	}

	updated := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path, `{"name":"Monty","archived":false}`), http.StatusOK)
	if updated.Species != "" || updated.LocationId != nil || updated.AcquiredOn != nil || updated.Notes != "" || updated.CreatedAt != created.CreatedAt {
		t.Errorf("PUT should replace fields: %+v", updated)
	}

	if resp := e.call(c, http.MethodDelete, path, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	expectProblem(t, e.call(c, http.MethodGet, path, ""), http.StatusNotFound)
}

func TestPlantArchive(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Fern"}`), http.StatusCreated)
	path := "/api/v1/plants/" + p.Id
	archivedAt := func() string {
		t.Helper()
		row, err := e.db.Reader().GetPlant(context.Background(), p.Id)
		if err != nil {
			t.Fatal(err)
		}
		return row.ArchivedAt.String
	}
	names := func(archived string) []string {
		t.Helper()
		list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants"+archived, ""), http.StatusOK)
		var out []string
		for _, it := range list.Items {
			out = append(out, it.Name)
		}
		return out
	}

	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path, `{"name":"Fern","archived":true}`), http.StatusOK); !got.Archived {
		t.Fatalf("not archived: %+v", got)
	}
	first := archivedAt()
	if first != "2026-05-01T09:00:00Z" {
		t.Errorf("archived_at = %q", first)
	}
	if active, archived := names(""), names("?archived=true"); len(active) != 0 || len(archived) != 1 {
		t.Errorf("active = %v, archived = %v", active, archived)
	}

	// Editing an archived plant keeps the original archive time.
	e.clock.t = e.clock.t.Add(time.Hour)
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path, `{"name":"Fern","notes":"Dormant","archived":true}`), http.StatusOK)
	if got := archivedAt(); got != first {
		t.Errorf("archived_at changed to %q", got)
	}

	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path, `{"name":"Fern","archived":false}`), http.StatusOK); got.Archived {
		t.Fatalf("still archived: %+v", got)
	}
	if active, archived := names("?archived=false"), names("?archived=true"); len(active) != 1 || len(archived) != 0 {
		t.Errorf("active = %v, archived = %v", active, archived)
	}
}

func TestPlantValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Fern"}`), http.StatusCreated)

	tests := []struct {
		name, method, path, body string
		status                   int
		detail                   string
	}{
		{"empty name", http.MethodPost, "", `{"name":""}`, http.StatusBadRequest, "Name is required."},
		{"whitespace name", http.MethodPost, "", `{"name":" \t "}`, http.StatusBadRequest, "Name is required."},
		{"name too long", http.MethodPost, "", `{"name":"` + strings.Repeat("é", 81) + `"}`, http.StatusBadRequest, "Name must be 80 characters or fewer."},
		{"species too long", http.MethodPost, "", `{"name":"x","species":"` + strings.Repeat("a", 121) + `"}`, http.StatusBadRequest, "Species must be 120 characters or fewer."},
		{"notes too long", http.MethodPost, "", `{"name":"x","notes":"` + strings.Repeat("a", 4001) + `"}`, http.StatusBadRequest, "Notes must be 4000 characters or fewer."},
		{"unknown location on create", http.MethodPost, "", `{"name":"x","location_id":"nope"}`, http.StatusBadRequest, "That location does not exist."},
		{"unknown location on update", http.MethodPut, "/" + p.Id, `{"name":"x","location_id":"nope","archived":false}`, http.StatusBadRequest, "That location does not exist."},
		{"bad date", http.MethodPost, "", `{"name":"x","acquired_on":"2026-02-30"}`, http.StatusBadRequest, "The request body is not valid JSON for this endpoint."},
		{"empty update name", http.MethodPut, "/" + p.Id, `{"name":"","archived":false}`, http.StatusBadRequest, "Name is required."},
		{"get unknown id", http.MethodGet, "/nope", "", http.StatusNotFound, "Plant not found."},
		{"update unknown id", http.MethodPut, "/nope", `{"name":"x","archived":false}`, http.StatusNotFound, "Plant not found."},
		{"delete unknown id", http.MethodDelete, "/nope", "", http.StatusNotFound, "Plant not found."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := e.call(c, tt.method, "/api/v1/plants"+tt.path, tt.body)
			if got := expectProblem(t, resp, tt.status); got != tt.detail {
				t.Errorf("detail = %q, want %q", got, tt.detail)
			}
		})
	}
	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id, ""), http.StatusOK); !reflect.DeepEqual(got, p) {
		t.Errorf("failed update changed the plant: %+v", got)
	}
}

func TestPlantMutationsNeedSameOrigin(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Fern"}`), http.StatusCreated)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		resp := e.do(request{method: method, path: "/api/v1/plants/" + p.Id, body: `{"name":"x","archived":true}`,
			cookie: c, headers: map[string]string{"Origin": "https://evil.example"}})
		expectProblem(t, resp, http.StatusForbidden)
	}
}
