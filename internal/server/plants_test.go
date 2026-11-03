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
		{"no name or species", http.MethodPost, "", `{"name":""}`, http.StatusBadRequest, "Give the plant a name or a species."},
		{"whitespace name and species", http.MethodPost, "", `{"name":" \t ","species":"  "}`, http.StatusBadRequest, "Give the plant a name or a species."},
		{"name too long", http.MethodPost, "", `{"name":"` + strings.Repeat("é", 81) + `"}`, http.StatusBadRequest, "Name must be 80 characters or fewer."},
		{"species too long", http.MethodPost, "", `{"name":"x","species":"` + strings.Repeat("a", 121) + `"}`, http.StatusBadRequest, "Species must be 120 characters or fewer."},
		{"notes too long", http.MethodPost, "", `{"name":"x","notes":"` + strings.Repeat("a", 4001) + `"}`, http.StatusBadRequest, "Notes must be 4000 characters or fewer."},
		{"unknown location on create", http.MethodPost, "", `{"name":"x","location_id":"nope"}`, http.StatusBadRequest, "That location does not exist."},
		{"unknown location on update", http.MethodPut, "/" + p.Id, `{"name":"x","location_id":"nope","archived":false}`, http.StatusBadRequest, "That location does not exist."},
		{"unknown pot type", http.MethodPost, "", `{"name":"x","pot_type":"glass"}`, http.StatusBadRequest, "Pot type must be plastic, terracotta, ceramic, fabric, self_watering or other."},
		{"pot too small", http.MethodPost, "", `{"name":"x","pot_size_in":0.4}`, http.StatusBadRequest, "Pot size must be 0.5 to 48 inches."},
		{"pot too large", http.MethodPut, "/" + p.Id, `{"name":"x","pot_size_in":48.5,"archived":false}`, http.StatusBadRequest, "Pot size must be 0.5 to 48 inches."},
		{"soil too long", http.MethodPost, "", `{"name":"x","soil":"` + strings.Repeat("a", 121) + `"}`, http.StatusBadRequest, "Soil must be 120 characters or fewer."},
		{"drainage not a bool", http.MethodPost, "", `{"name":"x","drainage":"yes"}`, http.StatusBadRequest, "The request body is not valid JSON for this endpoint."},
		{"bad date", http.MethodPost, "", `{"name":"x","acquired_on":"2026-02-30"}`, http.StatusBadRequest, "The request body is not valid JSON for this endpoint."},
		{"empty update name", http.MethodPut, "/" + p.Id, `{"name":"","archived":false}`, http.StatusBadRequest, "Give the plant a name or a species."},
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

func TestPlantPotAndSoil(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()

	bare := e.newPlant(c, "Aloe")
	if bare.PotType != nil || bare.PotSizeIn != nil || bare.Drainage != nil || bare.Soil != "" {
		t.Errorf("unrecorded pot = %+v", bare)
	}

	p := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants",
		`{"name":"Monty","pot_type":"self_watering","pot_size_in":0.5,"drainage":false,"soil":"  Aroid mix  "}`), http.StatusCreated)
	if p.PotType == nil || *p.PotType != apigen.SelfWatering || p.PotSizeIn == nil || *p.PotSizeIn != 0.5 ||
		p.Drainage == nil || *p.Drainage || p.Soil != "Aroid mix" {
		t.Errorf("created = %+v", p)
	}
	path := "/api/v1/plants/" + p.Id
	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, path, ""), http.StatusOK); !reflect.DeepEqual(got, p) {
		t.Errorf("get = %+v", got)
	}

	got := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path,
		`{"name":"Monty","pot_type":"terracotta","pot_size_in":48,"drainage":true,"soil":"Cactus mix","archived":false}`), http.StatusOK)
	if *got.PotType != apigen.Terracotta || *got.PotSizeIn != 48 || !*got.Drainage || got.Soil != "Cactus mix" {
		t.Errorf("updated = %+v", got)
	}
	list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	if len(list.Items) != 2 || !reflect.DeepEqual(list.Items[1], got) {
		t.Errorf("list = %+v", list.Items)
	}

	// PUT replaces: omitted pot fields go back to "not recorded".
	cleared := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, path, `{"name":"Monty","archived":false}`), http.StatusOK)
	if cleared.PotType != nil || cleared.PotSizeIn != nil || cleared.Drainage != nil || cleared.Soil != "" {
		t.Errorf("cleared = %+v", cleared)
	}
}

func TestPlantWithoutName(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	snake := e.newSpecies(c, `{"scientific_name":"Dracaena trifasciata","common_name":"Snake plant"}`)
	create := func(body string) apigen.Plant {
		t.Helper()
		return expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", body), http.StatusCreated)
	}

	linked := create(`{"name":"","species":"Dracaena trifasciata","species_id":"` + snake.Id + `"}`)
	if linked.Name != "" || linked.DisplayName != "Snake plant" {
		t.Errorf("linked: name %q, display %q; want the species' common name", linked.Name, linked.DisplayName)
	}
	typed := create(`{"name":" ","species":" monstera "}`)
	if typed.Name != "" || typed.Species != "monstera" || typed.DisplayName != "monstera" {
		t.Errorf("free text: name %q, display %q", typed.Name, typed.DisplayName)
	}
	named := create(`{"name":"Aloe"}`)
	if named.DisplayName != "Aloe" {
		t.Errorf("named: display %q", named.DisplayName)
	}

	// The list sorts by the name shown, not the empty name column.
	list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	var got []string
	for _, p := range list.Items {
		got = append(got, p.DisplayName)
	}
	if want := []string{"Aloe", "monstera", "Snake plant"}; !reflect.DeepEqual(got, want) {
		t.Errorf("list order = %q, want %q", got, want)
	}

	// Single-plant reads and Today name it the same way.
	if got := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+linked.Id, ""), http.StatusOK); got.DisplayName != "Snake plant" {
		t.Errorf("get: display %q", got.DisplayName)
	}
	expectJSON[apigen.CareTask](t, e.call(c, http.MethodPost, "/api/v1/plants/"+linked.Id+"/tasks",
		`{"kind":"water","interval_days":7,"start_on":"`+e.clock.t.Format(time.DateOnly)+`"}`), http.StatusCreated)
	today := expectJSON[apigen.Today](t, e.call(c, http.MethodGet, "/api/v1/today?date="+e.clock.t.Format(time.DateOnly), ""), http.StatusOK)
	if len(today.Due) != 1 || today.Due[0].Plant.DisplayName != "Snake plant" {
		t.Errorf("today due = %+v", today.Due)
	}
}
