package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
)

func TestLocationsCRUD(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()

	created := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations",
		`{"name":"  Kitchen window ","light":"bright_indirect","notes":"East facing"}`), http.StatusCreated)
	if created.Id == "" || created.Name != "Kitchen window" || created.Light == nil || *created.Light != apigen.BrightIndirect ||
		created.Notes != "East facing" || created.PlantCount != 0 {
		t.Errorf("created = %+v", created)
	}
	expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"bathroom"}`), http.StatusCreated)

	list := expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[0].Name != "bathroom" || list.Items[1].Id != created.Id {
		t.Errorf("list not ordered by name ignoring case: %+v", list.Items)
	}

	path := "/api/v1/locations/" + created.Id
	updated := expectJSON[apigen.Location](t, e.call(c, http.MethodPut, path, `{"name":"Living room","light":null}`), http.StatusOK)
	if updated.Id != created.Id || updated.Name != "Living room" || updated.Light != nil || updated.Notes != "" {
		t.Errorf("updated = %+v", updated)
	}
	// Renaming to its own name in another case is not a conflict.
	expectJSON[apigen.Location](t, e.call(c, http.MethodPut, path, `{"name":"LIVING ROOM"}`), http.StatusOK)

	if resp := e.call(c, http.MethodDelete, path, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	list = expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if len(list.Items) != 1 {
		t.Errorf("after delete: %+v", list.Items)
	}
}

func TestLocationPlantCountAndDelete(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	loc := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Shelf"}`), http.StatusCreated)
	body := `{"name":"Pothos","location_id":"` + loc.Id + `"}`
	keep := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", body), http.StatusCreated)
	gone := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", body), http.StatusCreated)
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+gone.Id,
		`{"name":"Pothos","location_id":"`+loc.Id+`","archived":true}`), http.StatusOK)

	list := expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if len(list.Items) != 1 || list.Items[0].PlantCount != 1 {
		t.Errorf("plant_count should exclude archived plants: %+v", list.Items)
	}

	if resp := e.call(c, http.MethodDelete, "/api/v1/locations/"+loc.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	for _, id := range []string{keep.Id, gone.Id} {
		p := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+id, ""), http.StatusOK)
		if p.LocationId != nil {
			t.Errorf("plant %s still placed in deleted location", id)
		}
	}
}

func TestLocationValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	existing := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Office"}`), http.StatusCreated)
	other := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Hall"}`), http.StatusCreated)

	tests := []struct {
		name, method, path, body string
		status                   int
		detail                   string
	}{
		{"empty name", http.MethodPost, "", `{"name":""}`, http.StatusBadRequest, "Name is required."},
		{"whitespace name", http.MethodPost, "", `{"name":"   "}`, http.StatusBadRequest, "Name is required."},
		{"name too long", http.MethodPost, "", `{"name":"` + strings.Repeat("a", 61) + `"}`, http.StatusBadRequest, "Name must be 60 characters or fewer."},
		{"notes too long", http.MethodPost, "", `{"name":"Den","notes":"` + strings.Repeat("a", 2001) + `"}`, http.StatusBadRequest, "Notes must be 2000 characters or fewer."},
		{"bad light", http.MethodPost, "", `{"name":"Den","light":"dim"}`, http.StatusBadRequest, "Light must be low, medium, bright_indirect or direct."},
		{"duplicate on create", http.MethodPost, "", `{"name":" office "}`, http.StatusConflict, "A location with that name already exists."},
		{"duplicate on rename", http.MethodPut, "/" + other.Id, `{"name":"OFFICE"}`, http.StatusConflict, "A location with that name already exists."},
		{"update unknown id", http.MethodPut, "/nope", `{"name":"Den"}`, http.StatusNotFound, "Location not found."},
		{"delete unknown id", http.MethodDelete, "/nope", "", http.StatusNotFound, "Location not found."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := e.call(c, tt.method, "/api/v1/locations"+tt.path, tt.body)
			if got := expectProblem(t, resp, tt.status); got != tt.detail {
				t.Errorf("detail = %q, want %q", got, tt.detail)
			}
		})
	}
	list := expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[1].Name != existing.Name {
		t.Errorf("failed requests changed data: %+v", list.Items)
	}
}

func TestPlantsAndLocationsRequireAuth(t *testing.T) {
	e := newEnv(t, envOptions{})
	tests := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/locations"},
		{http.MethodPost, "/api/v1/locations"},
		{http.MethodPut, "/api/v1/locations/x"},
		{http.MethodDelete, "/api/v1/locations/x"},
		{http.MethodGet, "/api/v1/plants"},
		{http.MethodPost, "/api/v1/plants"},
		{http.MethodGet, "/api/v1/plants/x"},
		{http.MethodPut, "/api/v1/plants/x"},
		{http.MethodDelete, "/api/v1/plants/x"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			expectProblem(t, e.call(nil, tt.method, tt.path, `{"name":"x","archived":false}`), http.StatusUnauthorized)
		})
	}
}
