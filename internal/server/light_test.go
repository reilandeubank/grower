package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
)

// windowsOf renders a location's windows and what they suggest as one comparable string.
func windowsOf(l apigen.Location) string {
	s := ""
	for _, w := range l.Windows {
		s += fmt.Sprintf("%d%s ", w.Count, w.Direction)
	}
	if l.SuggestedLight == nil {
		return s + "-> none"
	}
	return s + "-> " + string(*l.SuggestedLight)
}

func effective(p apigen.Plant) string {
	if p.EffectiveLight == nil {
		return "none"
	}
	return string(p.EffectiveLight.Level) + " from " + string(p.EffectiveLight.Source)
}

func (e *env) setHemisphere(c *http.Cookie, h string) {
	e.t.Helper()
	expectJSON[apigen.Settings](e.t, e.call(c, http.MethodPut, "/api/v1/settings",
		`{"timezone":"America/Chicago","digest_time":"08:00","hemisphere":"`+h+`"}`), http.StatusOK)
}

func TestLocationWindows(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	const sunny = `{"name":"Sunroom","windows":[{"direction":"s","count":1},{"direction":"n","count":1}]}`
	loc := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", sunny), http.StatusCreated)
	if got := windowsOf(loc); got != "1n 1s -> direct" || loc.Light != nil {
		t.Errorf("created: %s, light %v", got, loc.Light)
	}
	path := "/api/v1/locations/" + loc.Id
	bare := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Closet"}`), http.StatusCreated)
	if got := windowsOf(bare); got != "-> none" || bare.Windows == nil {
		t.Errorf("no windows: %s, %v", got, bare.Windows)
	}

	// The southern hemisphere's sun is to the north.
	e.setHemisphere(c, "south")
	list := expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if got := windowsOf(list.Items[1]); got != "1n 1s -> direct" {
		t.Errorf("south, n and s: %s", got)
	}
	// Update replaces the whole set; two pole-facing windows raise medium.
	upd := expectJSON[apigen.Location](t, e.call(c, http.MethodPut, path,
		`{"name":"Sunroom","windows":[{"direction":"sw","count":1},{"direction":"s","count":1}]}`), http.StatusOK)
	if got := windowsOf(upd); got != "1s 1sw -> bright_indirect" {
		t.Errorf("south, s and sw: %s", got)
	}
	e.setHemisphere(c, "north")
	list = expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if got := windowsOf(list.Items[1]); got != "1s 1sw -> direct" {
		t.Errorf("north, s and sw: %s", got)
	}

	// Invalid windows, and a failed update, leave the set alone.
	tests := []struct{ name, method, path, body, detail string }{
		{"duplicate direction", http.MethodPost, "", `{"name":"Den","windows":[{"direction":"e","count":1},{"direction":"e","count":2}]}`, "List each window direction once."},
		{"zero count", http.MethodPut, "/" + loc.Id, `{"name":"Sunroom","windows":[{"direction":"e","count":0}]}`, "Window count must be 1 to 9."},
		{"ten windows", http.MethodPut, "/" + loc.Id, `{"name":"Sunroom","windows":[{"direction":"e","count":10}]}`, "Window count must be 1 to 9."},
		{"bad direction", http.MethodPost, "", `{"name":"Den","windows":[{"direction":"up","count":1}]}`, "Window direction must be n, ne, e, se, s, sw, w or nw."},
	}
	for _, tt := range tests {
		if got := expectProblem(t, e.call(c, tt.method, "/api/v1/locations"+tt.path, tt.body), http.StatusBadRequest); got != tt.detail {
			t.Errorf("%s: detail = %q, want %q", tt.name, got, tt.detail)
		}
	}
	expectProblem(t, e.call(c, http.MethodPut, path, `{"name":"closet","windows":[{"direction":"n","count":1}]}`), http.StatusConflict)
	list = expectJSON[apigen.LocationList](t, e.call(c, http.MethodGet, "/api/v1/locations", ""), http.StatusOK)
	if len(list.Items) != 2 || windowsOf(list.Items[1]) != "1s 1sw -> direct" {
		t.Errorf("failed requests changed data: %+v", list.Items)
	}

	// Omitting windows clears them.
	cleared := expectJSON[apigen.Location](t, e.call(c, http.MethodPut, path, `{"name":"Sunroom","light":"low"}`), http.StatusOK)
	if got := windowsOf(cleared); got != "-> none" || cleared.Light == nil || *cleared.Light != apigen.Low {
		t.Errorf("cleared: %s", got)
	}
}

func TestPlantEffectiveLight(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	south := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations",
		`{"name":"South room","windows":[{"direction":"s","count":1}]}`), http.StatusCreated)
	hall := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations",
		`{"name":"Hall","light":"low","windows":[{"direction":"s","count":1}]}`), http.StatusCreated)
	create := func(body string) apigen.Plant {
		t.Helper()
		return expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", body), http.StatusCreated)
	}

	plants := []struct {
		body, want string
	}{
		{`{"name":"a nothing known","window_distance":"near"}`, "none"},
		{`{"name":"b own light","light":"medium","location_id":"` + south.Id + `","grow_light_hours":12}`, "medium from plant"},
		{`{"name":"c sunny window","location_id":"` + south.Id + `","window_distance":"at_window"}`, "direct from windows"},
		{`{"name":"d across sunny room","location_id":"` + south.Id + `","window_distance":"across_room"}`, "medium from windows"},
		{`{"name":"e hall","location_id":"` + hall.Id + `"}`, "low from location"},
		{`{"name":"f hall grow light","location_id":"` + hall.Id + `","grow_light_hours":10}`, "bright_indirect from grow_light"},
		{`{"name":"g short grow light","grow_light_hours":9}`, "medium from grow_light"},
		{`{"name":"h grow light not needed","location_id":"` + south.Id + `","grow_light_hours":16}`, "direct from windows"},
	}
	ids := make([]string, len(plants))
	for i, p := range plants {
		created := create(p.body)
		ids[i] = created.Id
		if got := effective(created); got != p.want {
			t.Errorf("create %s: %s, want %s", p.body, got, p.want)
		}
		if got := effective(expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+created.Id, ""), http.StatusOK)); got != p.want {
			t.Errorf("get %s: %s, want %s", p.body, got, p.want)
		}
	}
	b := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+ids[1], ""), http.StatusOK)
	if b.Light == nil || *b.Light != apigen.Medium || b.GrowLightHours == nil || *b.GrowLightHours != 12 || b.WindowDistance != nil {
		t.Errorf("stored fields: %+v", b)
	}

	// The list, Today and cover responses carry it too; a southern hemisphere dims the south room.
	e.newTask(c, ids[3], `{"kind":"water","interval_days":1}`)
	e.setHemisphere(c, "south")
	list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	want := []string{"none", "medium from plant", "medium from windows", "low from windows", "low from location",
		"bright_indirect from grow_light", "medium from grow_light", "bright_indirect from grow_light"}
	for i, p := range list.Items {
		if got := effective(p); got != want[i] {
			t.Errorf("list %s: %s, want %s", p.Name, got, want[i])
		}
	}
	if items := e.today(c).Due; len(items) != 1 || effective(items[0].Plant) != "low from windows" {
		t.Errorf("today: %+v", items)
	}
	covered := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+ids[3]+"/cover", `{"photo_id":null}`), http.StatusOK)
	if got := effective(covered); got != "low from windows" {
		t.Errorf("cover: %s", got)
	}

	// Archived plants, and updates, too; updating replaces the light fields.
	archived := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+ids[2],
		`{"name":"c sunny window","location_id":"`+south.Id+`","window_distance":"near","grow_light_hours":10,"archived":true}`), http.StatusOK)
	if got := effective(archived); got != "bright_indirect from grow_light" {
		t.Errorf("update: %s", got)
	}
	gone := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants?archived=true", ""), http.StatusOK)
	if len(gone.Items) != 1 || effective(gone.Items[0]) != "bright_indirect from grow_light" {
		t.Errorf("archived list: %+v", gone.Items)
	}
}

func TestPlantLightValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Fern")
	tests := []struct{ name, fields, detail string }{
		{"bad light", `"light":"dim"`, "Light must be low, medium, bright_indirect or direct."},
		{"bad distance", `"window_distance":"far"`, "Window distance must be at_window, near or across_room."},
		{"no hours", `"grow_light_hours":0`, "Grow light hours must be 1 to 24."},
		{"too many hours", `"grow_light_hours":25`, "Grow light hours must be 1 to 24."},
	}
	for _, tt := range tests {
		if got := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Fern",`+tt.fields+`}`), http.StatusBadRequest); got != tt.detail {
			t.Errorf("create, %s: detail = %q, want %q", tt.name, got, tt.detail)
		}
		if got := expectProblem(t, e.call(c, http.MethodPut, "/api/v1/plants/"+p.Id, `{"name":"Fern","archived":false,`+tt.fields+`}`), http.StatusBadRequest); got != tt.detail {
			t.Errorf("update, %s: detail = %q, want %q", tt.name, got, tt.detail)
		}
	}
}
