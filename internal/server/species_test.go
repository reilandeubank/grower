package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/species"
)

// fakeProvider answers every search with its candidates and every import with its care.
type fakeProvider struct {
	source, name string
	candidates   []species.Candidate
	care         species.Care
	err          error
}

func (f *fakeProvider) Source() string { return f.source }
func (f *fakeProvider) Name() string   { return f.name }
func (f *fakeProvider) Search(context.Context, string) ([]species.Candidate, error) {
	return f.candidates, f.err
}
func (f *fakeProvider) Details(context.Context, string) (species.Care, error) { return f.care, f.err }

func ptr[T any](v T) *T { return &v }

func (e *env) newSpecies(c *http.Cookie, body string) apigen.Species {
	e.t.Helper()
	return expectJSON[apigen.Species](e.t, e.call(c, http.MethodPost, "/api/v1/species", body), http.StatusCreated)
}

func TestSpeciesCRUD(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()

	s := e.newSpecies(c, `{"scientific_name":"  Monstera deliciosa ","common_name":"Swiss cheese plant","water_interval_days":7,"light":"bright_indirect","water_at":3}`)
	if s.ScientificName != "Monstera deliciosa" || s.CommonName != "Swiss cheese plant" || s.Source != apigen.SpeciesSourceManual ||
		s.SourceRef != "" || *s.WaterIntervalDays != 7 || *s.Light != apigen.BrightIndirect || *s.WaterAt != 3 ||
		s.FetchedAt != nil || !s.UserEdited || s.PlantCount != 0 {
		t.Errorf("created = %+v", s)
	}
	fern := e.newSpecies(c, `{"scientific_name":"Nephrolepis exaltata","common_name":"Boston fern"}`)
	if fern.WaterIntervalDays != nil || fern.Light != nil || fern.WaterAt != nil {
		t.Errorf("fern = %+v", fern)
	}
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species", `{"scientific_name":"MONSTERA DELICIOSA"}`), http.StatusConflict); d != "That species is already saved." {
		t.Errorf("duplicate detail = %q", d)
	}

	path := "/api/v1/species/" + s.Id
	if got := expectJSON[apigen.Species](t, e.call(c, http.MethodGet, path, ""), http.StatusOK); got.Id != s.Id {
		t.Errorf("get = %+v", got)
	}
	expectProblem(t, e.call(c, http.MethodGet, "/api/v1/species/nope", ""), http.StatusNotFound)

	upd := expectJSON[apigen.Species](t, e.call(c, http.MethodPut, path, `{"scientific_name":"Monstera deliciosa","water_interval_days":10,"notes":"Likes a pole"}`), http.StatusOK)
	if upd.CommonName != "" || *upd.WaterIntervalDays != 10 || upd.Light != nil || upd.WaterAt != nil || upd.Notes != "Likes a pole" || !upd.UserEdited {
		t.Errorf("updated = %+v", upd)
	}
	expectProblem(t, e.call(c, http.MethodPut, path, `{"scientific_name":"nephrolepis EXALTATA"}`), http.StatusConflict)
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/species/nope", `{"scientific_name":"X"}`), http.StatusNotFound)

	names := func(q string) []string {
		t.Helper()
		list := expectJSON[apigen.SpeciesList](t, e.call(c, http.MethodGet, "/api/v1/species"+q, ""), http.StatusOK)
		out := []string{}
		for _, it := range list.Items {
			out = append(out, it.ScientificName)
		}
		return out
	}
	for q, want := range map[string]int{"": 2, "?q=MONSTERA": 1, "?q=boston": 1, "?q=%25": 0, "?q=zz": 0} {
		if got := names(q); len(got) != want {
			t.Errorf("list %q = %v, want %d", q, got, want)
		}
	}
	if got := names(""); got[0] != "Monstera deliciosa" {
		t.Errorf("not ordered by scientific name: %v", got)
	}
	expectProblem(t, e.call(nil, http.MethodGet, "/api/v1/species", ""), http.StatusUnauthorized)
}

func TestSpeciesValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	long := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = 'a'
		}
		return string(b)
	}
	for body, want := range map[string]string{
		`{"scientific_name":"  "}`:                                  "Scientific name is required.",
		`{"scientific_name":"` + long(121) + `"}`:                   "Scientific name must be 120 characters or fewer.",
		`{"scientific_name":"X","common_name":"` + long(121) + `"}`: "Common name must be 120 characters or fewer.",
		`{"scientific_name":"X","water_interval_days":0}`:           "Water interval must be 1 to 365 days.",
		`{"scientific_name":"X","water_interval_days":366}`:         "Water interval must be 1 to 365 days.",
		`{"scientific_name":"X","light":"dark"}`:                    "Light must be low, medium, bright_indirect or direct.",
		`{"scientific_name":"X","water_at":10}`:                     "Water-at must be between 1 and 9.",
		`{"scientific_name":"X","notes":"` + long(2001) + `"}`:      "Notes must be 2000 characters or fewer.",
	} {
		if got := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species", body), http.StatusBadRequest); got != want {
			t.Errorf("%.60s: detail %q, want %q", body, got, want)
		}
	}
}

func TestPlantSpeciesID(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	s := e.newSpecies(c, `{"scientific_name":"Monstera deliciosa"}`)

	p := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Monty","species_id":"`+s.Id+`"}`), http.StatusCreated)
	if p.SpeciesId == nil || *p.SpeciesId != s.Id {
		t.Errorf("plant = %+v", p)
	}
	old := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"Old","species_id":"`+s.Id+`"}`), http.StatusCreated)
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+old.Id, `{"name":"Old","species_id":"`+s.Id+`","archived":true}`), http.StatusOK)
	if got := expectJSON[apigen.Species](t, e.call(c, http.MethodGet, "/api/v1/species/"+s.Id, ""), http.StatusOK); got.PlantCount != 1 {
		t.Errorf("plant_count = %d, want 1 (archived plants don't count)", got.PlantCount)
	}

	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"X","species_id":"nope"}`), http.StatusBadRequest); d != "That species does not exist." {
		t.Errorf("create detail = %q", d)
	}
	if d := expectProblem(t, e.call(c, http.MethodPut, "/api/v1/plants/"+p.Id, `{"name":"Monty","species_id":"nope","archived":false}`), http.StatusBadRequest); d != "That species does not exist." {
		t.Errorf("update detail = %q", d)
	}
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"X","species_id":"`+s.Id+`","location_id":"nope"}`), http.StatusBadRequest); d != "That location does not exist." {
		t.Errorf("location detail = %q", d)
	}
	unlinked := expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+p.Id, `{"name":"Monty","archived":false}`), http.StatusOK)
	if unlinked.SpeciesId != nil {
		t.Errorf("PUT without species_id should unlink: %+v", unlinked)
	}
}

func TestSpeciesLookup(t *testing.T) {
	plantbook := &fakeProvider{source: "openplantbook", name: "Open Plantbook", candidates: []species.Candidate{
		{Source: "openplantbook", Ref: "monstera deliciosa", ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant"},
		{Source: "openplantbook", Ref: "monstera deliciosa 2", ScientificName: "MONSTERA DELICIOSA"},
		{Source: "openplantbook", Ref: "monstera adansonii", ScientificName: "Monstera adansonii"},
	}}
	perenual := &fakeProvider{source: "perenual", name: "Perenual", candidates: []species.Candidate{
		{Source: "perenual", Ref: "2961", ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant"},
	}}
	e := newEnv(t, envOptions{species: []species.Provider{plantbook, perenual}})
	c := e.loginCookie()
	saved := e.newSpecies(c, `{"scientific_name":"monstera Deliciosa"}`)

	got := expectJSON[apigen.SpeciesLookup](t, e.call(c, http.MethodGet, "/api/v1/species/lookup?q=monstera", ""), http.StatusOK)
	if len(got.Providers) != 2 || !got.Providers[0].Ok || got.Providers[0].Source != "openplantbook" || !got.Providers[1].Ok || got.Providers[1].Detail != nil {
		t.Errorf("providers = %+v", got.Providers)
	}
	if len(got.Items) != 3 {
		t.Fatalf("items = %+v", got.Items)
	}
	for i, want := range []struct{ source, ref, saved string }{
		{"openplantbook", "monstera deliciosa", saved.Id}, {"openplantbook", "monstera adansonii", ""}, {"perenual", "2961", saved.Id},
	} {
		it := got.Items[i]
		if string(it.Source) != want.source || it.Ref != want.ref || (it.SavedId == nil) != (want.saved == "") ||
			(it.SavedId != nil && *it.SavedId != want.saved) {
			t.Errorf("item %d = %+v, want %+v", i, it, want)
		}
	}

	// One provider failing still returns the other's candidates (the query is new, so not cached).
	perenual.err = species.ErrDailyLimit
	plantbook.err = errors.New("timeout")
	got = expectJSON[apigen.SpeciesLookup](t, e.call(c, http.MethodGet, "/api/v1/species/lookup?q=monstera+d", ""), http.StatusOK)
	if len(got.Items) != 0 || got.Providers[0].Ok || *got.Providers[0].Detail != "Couldn't reach Open Plantbook. Try again later." ||
		got.Providers[1].Ok || *got.Providers[1].Detail != "Perenual's daily limit is used up. Try again tomorrow." {
		t.Errorf("failed lookup = %+v", got)
	}
	plantbook.err = nil
	got = expectJSON[apigen.SpeciesLookup](t, e.call(c, http.MethodGet, "/api/v1/species/lookup?q=Monstera%20D", ""), http.StatusOK)
	if len(got.Items) != 2 || !got.Providers[0].Ok || got.Providers[1].Ok {
		t.Errorf("partial lookup = %+v", got)
	}

	for _, q := range []string{"", "m", "%20m%20"} {
		expectProblem(t, e.call(c, http.MethodGet, "/api/v1/species/lookup?q="+q, ""), http.StatusBadRequest)
	}
}

func TestSpeciesLookupWithoutProviders(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	got := expectJSON[apigen.SpeciesLookup](t, e.call(c, http.MethodGet, "/api/v1/species/lookup?q=monstera", ""), http.StatusOK)
	if got.Providers == nil || len(got.Providers) != 0 || got.Items == nil || len(got.Items) != 0 {
		t.Errorf("lookup = %+v", got)
	}
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species/import", `{"source":"perenual","ref":"1"}`), http.StatusBadRequest); d != "That provider isn't set up on this server." {
		t.Errorf("detail = %q", d)
	}
}

func TestSpeciesImport(t *testing.T) {
	p := &fakeProvider{source: "perenual", name: "Perenual", care: species.Care{
		ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant", WaterIntervalDays: ptr(9),
		Light: ptr("bright_indirect"), Raw: []byte(`{"id":2961}`),
	}}
	e := newEnv(t, envOptions{species: []species.Provider{p}})
	c := e.loginCookie()
	raw := func(id string) string {
		t.Helper()
		var s string
		if err := e.db.Read.QueryRow("SELECT raw FROM species WHERE id = ?", id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	imp := func() apigen.Species {
		t.Helper()
		return expectJSON[apigen.Species](t, e.call(c, http.MethodPost, "/api/v1/species/import", `{"source":"perenual","ref":"2961"}`), http.StatusOK)
	}

	s := imp()
	if s.ScientificName != "Monstera deliciosa" || s.CommonName != "Swiss cheese plant" || s.Source != apigen.SpeciesSourcePerenual ||
		s.SourceRef != "2961" || *s.WaterIntervalDays != 9 || *s.Light != apigen.BrightIndirect || s.WaterAt != nil ||
		s.FetchedAt == nil || !s.FetchedAt.Equal(e.clock.t) || s.UserEdited || raw(s.Id) != `{"id":2961}` {
		t.Errorf("inserted = %+v", s)
	}

	// Not edited: a re-import (matched by name, ignoring case) takes the new values.
	e.clock.t = e.clock.t.Add(time.Hour)
	p.care.ScientificName, p.care.WaterIntervalDays, p.care.Raw = "MONSTERA DELICIOSA", ptr(6), []byte(`{"id":2961,"v":2}`)
	again := imp()
	if again.Id != s.Id || *again.WaterIntervalDays != 6 || !again.FetchedAt.Equal(e.clock.t) || raw(s.Id) != `{"id":2961,"v":2}` {
		t.Errorf("refreshed = %+v", again)
	}

	// Edited: the person's values stay; only raw and fetched_at move.
	expectJSON[apigen.Species](t, e.call(c, http.MethodPut, "/api/v1/species/"+s.Id, `{"scientific_name":"Monstera deliciosa","water_interval_days":12}`), http.StatusOK)
	e.clock.t = e.clock.t.Add(time.Hour)
	p.care.WaterIntervalDays, p.care.Raw = ptr(3), []byte(`{"id":2961,"v":3}`)
	kept := imp()
	if *kept.WaterIntervalDays != 12 || kept.Light != nil || !kept.UserEdited || !kept.FetchedAt.Equal(e.clock.t) || raw(s.Id) != `{"id":2961,"v":3}` {
		t.Errorf("edited re-import = %+v", kept)
	}

	p.err = errors.New("connection refused")
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species/import", `{"source":"perenual","ref":"1"}`), http.StatusBadGateway); d != "Couldn't reach Perenual. Try again later." {
		t.Errorf("failure detail = %q", d)
	}
	p.err = species.ErrDailyLimit
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species/import", `{"source":"perenual","ref":"1"}`), http.StatusBadGateway); d != "Perenual's daily limit is used up. Try again tomorrow." {
		t.Errorf("limit detail = %q", d)
	}
	for body, want := range map[string]string{
		`{"source":"trefle","ref":"1"}`:        "Source must be openplantbook or perenual.",
		`{"source":"perenual","ref":"  "}`:     "Ref must be 1 to 200 characters.",
		`{"source":"openplantbook","ref":"x"}`: "That provider isn't set up on this server.",
	} {
		if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/species/import", body), http.StatusBadRequest); d != want {
			t.Errorf("%s: detail %q, want %q", body, d, want)
		}
	}
}

func TestApplySpeciesCare(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	apply := func(plantID string) *http.Response {
		return e.call(c, http.MethodPost, "/api/v1/plants/"+plantID+"/apply-species", "")
	}
	link := func(name, speciesID string) apigen.Plant {
		t.Helper()
		return expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"`+name+`","species_id":"`+speciesID+`"}`), http.StatusCreated)
	}

	bare := e.newPlant(c, "Bare")
	if d := expectProblem(t, apply(bare.Id), http.StatusBadRequest); d != "Link a species to this plant first." {
		t.Errorf("detail = %q", d)
	}
	expectProblem(t, apply("nope"), http.StatusNotFound)
	empty := e.newSpecies(c, `{"scientific_name":"Unknownia"}`)
	if d := expectProblem(t, apply(link("Mystery", empty.Id).Id), http.StatusBadRequest); d != "This species has no care defaults yet." {
		t.Errorf("detail = %q", d)
	}

	full := e.newSpecies(c, `{"scientific_name":"Monstera deliciosa","water_interval_days":10,"water_at":4}`)

	// No water task: one is created, a plain one starting today.
	monty := link("Monty", full.Id)
	e.newTask(c, monty.Id, `{"kind":"fertilize","interval_days":30}`)
	got := expectJSON[apigen.CareTaskList](t, apply(monty.Id), http.StatusOK)
	if len(got.Items) != 2 {
		t.Fatalf("tasks = %+v", got.Items)
	}
	for _, task := range got.Items {
		if task.Kind == apigen.Water && (task.IntervalDays != 10 || task.MoistureCheck || task.StartOn.String() != "2026-05-01" || task.WaterAt != 4) {
			t.Errorf("created task = %+v", task)
		}
	}

	// The first water task is updated (water_at too, since it is a soil check); a later one isn't.
	fig := link("Fig", full.Id)
	first := e.newTask(c, fig.Id, `{"kind":"water","interval_days":5,"moisture_check":true,"water_at":2}`)
	e.clock.t = e.clock.t.Add(time.Minute)
	second := e.newTask(c, fig.Id, `{"kind":"water","interval_days":5}`)
	got = expectJSON[apigen.CareTaskList](t, apply(fig.Id), http.StatusOK)
	for _, task := range got.Items {
		switch task.Id {
		case first.Id:
			if task.IntervalDays != 10 || task.WaterAt != 4 || !task.MoistureCheck {
				t.Errorf("first = %+v", task)
			}
		case second.Id:
			if task.IntervalDays != 5 || task.WaterAt != 3 {
				t.Errorf("second = %+v", task)
			}
		}
	}

	// A species with only water_at leaves a plain task alone.
	wet := e.newSpecies(c, `{"scientific_name":"Pilea","water_at":5}`)
	pilea := link("Pilea", wet.Id)
	plain := e.newTask(c, pilea.Id, `{"kind":"water","interval_days":8}`)
	got = expectJSON[apigen.CareTaskList](t, apply(pilea.Id), http.StatusOK)
	if len(got.Items) != 1 || got.Items[0].Id != plain.Id || got.Items[0].IntervalDays != 8 || got.Items[0].WaterAt != 3 {
		t.Errorf("water_at only = %+v", got.Items)
	}
}
