package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

func (e *env) newMeasurement(c *http.Cookie, plantID, body string) apigen.Measurement {
	e.t.Helper()
	return expectJSON[apigen.Measurement](e.t, e.call(c, http.MethodPost, "/api/v1/plants/"+plantID+"/measurements", body), http.StatusCreated)
}

func (e *env) measurements(c *http.Cookie, plantID string) []apigen.Measurement {
	e.t.Helper()
	return expectJSON[apigen.MeasurementList](e.t, e.call(c, http.MethodGet, "/api/v1/plants/"+plantID+"/measurements", ""), http.StatusOK).Items
}

func (e *env) plant(c *http.Cookie, id string) apigen.Plant {
	e.t.Helper()
	return expectJSON[apigen.Plant](e.t, e.call(c, http.MethodGet, "/api/v1/plants/"+id, ""), http.StatusOK)
}

func TestCheckInTask(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monstera")

	task := e.newTask(c, p.Id, `{"kind":"check_in","label":"ignored","interval_days":30}`)
	if task.Kind != apigen.CheckIn || task.Label != "" || task.Seasonal || task.MoistureCheck || task.NextDue.String() != "2026-05-01" {
		t.Errorf("check-in = %+v", task)
	}
	if d := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/plants/"+p.Id+"/tasks",
		`{"kind":"check_in","interval_days":30,"moisture_check":true}`), http.StatusBadRequest); d != "Only watering tasks can use soil checks." {
		t.Errorf("soil check detail = %q", d)
	}
	// Due today like any task, and done the same way.
	if due := e.today(c).Due; len(due) != 1 || due[0].Task.Id != task.Id {
		t.Fatalf("today due = %+v", due)
	}
	if done := e.logEvent(c, task.Id, `{"kind":"done"}`); done.Task.NextDue.String() != "2026-05-31" {
		t.Errorf("after done = %+v", done.Task)
	}
}

func TestMeasurementCRUD(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monstera")
	if p.Growth != nil {
		t.Errorf("new plant growth = %+v", p.Growth)
	}

	m := e.newMeasurement(c, p.Id, `{"height_in":12.5,"stage":"young","note":"  New leaf  "}`)
	if m.Id == "" || m.PlantId != p.Id || m.MeasuredOn.String() != "2026-05-01" || *m.HeightIn != 12.5 || m.WidthIn != nil ||
		*m.Stage != apigen.Young || m.Note != "New leaf" || m.Photo != nil {
		t.Errorf("created = %+v", m)
	}

	updated := expectJSON[apigen.Measurement](t, e.call(c, http.MethodPut, "/api/v1/measurements/"+m.Id,
		`{"measured_on":"2026-04-20","width_in":8}`), http.StatusOK)
	// PUT replaces: omitted fields clear.
	if updated.MeasuredOn.String() != "2026-04-20" || updated.HeightIn != nil || *updated.WidthIn != 8 || updated.Stage != nil || updated.Note != "" {
		t.Errorf("updated = %+v", updated)
	}

	other := e.newMeasurement(c, p.Id, `{"note":"Repotted"}`)
	list := e.measurements(c, p.Id)
	if len(list) != 2 || list[0].Id != other.Id || list[1].Id != m.Id {
		t.Errorf("list = %+v", list)
	}

	if resp := e.call(c, http.MethodDelete, "/api/v1/measurements/"+other.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	expectProblem(t, e.call(c, http.MethodDelete, "/api/v1/measurements/"+other.Id, ""), http.StatusNotFound)
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/measurements/"+other.Id, `{"note":"x"}`), http.StatusNotFound)
	expectProblem(t, e.call(c, http.MethodGet, "/api/v1/plants/missing/measurements", ""), http.StatusNotFound)
	expectProblem(t, e.call(c, http.MethodPost, "/api/v1/plants/missing/measurements", `{"note":"x"}`), http.StatusNotFound)

	// Deleting the plant deletes its measurements.
	if resp := e.call(c, http.MethodDelete, "/api/v1/plants/"+p.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete plant status = %d", resp.StatusCode)
	}
	var n int
	if err := e.db.Read.QueryRow(`SELECT COUNT(*) FROM plant_measurements`).Scan(&n); err != nil || n != 0 {
		t.Errorf("measurements left: %d, %v", n, err)
	}
}

func TestMeasurementValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	e.clock.t = time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC) // April 30 in Chicago
	c := e.loginCookie()
	p := e.newPlant(c, "Monstera")
	m := e.newMeasurement(c, p.Id, `{"stage":"seedling"}`)
	if m.MeasuredOn.String() != "2026-04-30" {
		t.Errorf("default date = %s, want today in the user's time zone", m.MeasuredOn)
	}

	for _, tc := range []struct{ body, detail string }{
		{`{}`, "Add a size, a stage, a note or a photo."},
		{`{"note":"   ","measured_on":"2026-04-01"}`, "Add a size, a stage, a note or a photo."},
		{`{"note":"x","measured_on":"2026-05-01"}`, "Pick today or an earlier date."},
		{`{"height_in":0}`, "Sizes must be 0.1 to 600 inches."},
		{`{"height_in":0.05}`, "Sizes must be 0.1 to 600 inches."},
		{`{"width_in":600.5}`, "Sizes must be 0.1 to 600 inches."},
		{`{"width_in":-3}`, "Sizes must be 0.1 to 600 inches."},
		{`{"stage":"old"}`, "Stage must be seedling, young or mature."},
		{`{"note":"` + strings.Repeat("a", 2001) + `"}`, "Note must be 2000 characters or fewer."},
		{`{"photo_id":"missing"}`, "That photo does not exist."},
	} {
		for _, req := range []struct{ method, path string }{
			{http.MethodPost, "/api/v1/plants/" + p.Id + "/measurements"},
			{http.MethodPut, "/api/v1/measurements/" + m.Id},
		} {
			if d := expectProblem(t, e.call(c, req.method, req.path, tc.body), http.StatusBadRequest); d != tc.detail {
				t.Errorf("%s %s: detail = %q, want %q", req.method, tc.body, d, tc.detail)
			}
		}
	}
	// The limits themselves are fine, as is a measurement that is only a photo or a note.
	e.newMeasurement(c, p.Id, `{"height_in":0.1,"width_in":600,"measured_on":"2026-04-30","note":"`+strings.Repeat("a", 2000)+`"}`)
	e.newMeasurement(c, p.Id, `{"note":"Looking good"}`)
	if got := len(e.measurements(c, p.Id)); got != 3 {
		t.Errorf("%d measurements, want 3 (invalid ones must not be saved)", got)
	}
}

func TestMeasurementPhotos(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monstera")
	other := e.newPlant(c, "Fern")
	ph := e.newPhoto(c, p.Id, testPNG(t, 300, 200, 1), "2026-04-30T10:00:00Z")
	foreign := e.newPhoto(c, other.Id, testPNG(t, 300, 200, 2), "2026-04-30T10:00:00Z")

	m := e.newMeasurement(c, p.Id, `{"photo_id":"`+ph.Id+`"}`)
	if m.Photo == nil || m.Photo.Id != ph.Id || m.Photo.Urls.Small != "/media/photos/"+ph.Id+"/small" {
		t.Errorf("created photo = %+v", m.Photo)
	}
	if list := e.measurements(c, p.Id); len(list) != 1 || list[0].Photo == nil || list[0].Photo.Id != ph.Id {
		t.Errorf("listed = %+v", list)
	}
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/plants/" + p.Id + "/measurements"},
		{http.MethodPut, "/api/v1/measurements/" + m.Id},
	} {
		if d := expectProblem(t, e.call(c, req.method, req.path, `{"photo_id":"`+foreign.Id+`"}`), http.StatusBadRequest); d != "That photo belongs to another plant." {
			t.Errorf("%s: detail = %q", req.method, d)
		}
	}

	// Deleting the measurement keeps the photo; deleting the photo keeps the measurement.
	keep := e.newMeasurement(c, p.Id, `{"note":"With photo","photo_id":"`+ph.Id+`"}`)
	e.call(c, http.MethodDelete, "/api/v1/measurements/"+m.Id, "")
	expectJSON[apigen.PhotoList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/photos", ""), http.StatusOK)
	if resp := e.call(c, http.MethodDelete, "/api/v1/photos/"+ph.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete photo status = %d", resp.StatusCode)
	}
	if list := e.measurements(c, p.Id); len(list) != 1 || list[0].Id != keep.Id || list[0].Photo != nil || list[0].Note != "With photo" {
		t.Errorf("after photo delete = %+v", list)
	}
}

func TestPlantGrowth(t *testing.T) {
	e := newEnv(t, envOptions{})
	e.clock.t = time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)
	c := e.loginCookie()
	p := e.newPlant(c, "Monstera")
	bare := e.newPlant(c, "Fern")
	// Partial measurements: each value comes from the newest one that has it.
	e.newMeasurement(c, p.Id, `{"measured_on":"2026-05-01","height_in":10,"width_in":6,"stage":"seedling"}`)
	e.newMeasurement(c, p.Id, `{"measured_on":"2026-05-10","height_in":14}`)
	e.newMeasurement(c, p.Id, `{"measured_on":"2026-05-10","stage":"young"}`) // same day, later
	e.newMeasurement(c, p.Id, `{"measured_on":"2026-05-03","width_in":9}`)    // logged late, older date
	e.newMeasurement(c, p.Id, `{"measured_on":"2026-05-20","note":"Just a note"}`)

	growth := func(p apigen.Plant) string {
		g := p.Growth
		if g == nil {
			return "none"
		}
		v := func(f *float64) string {
			if f == nil {
				return "-"
			}
			return fmt.Sprint(*f)
		}
		stage := "-"
		if g.Stage != nil {
			stage = string(*g.Stage)
		}
		return strings.Join([]string{v(g.HeightIn), v(g.WidthIn), stage, g.MeasuredOn.String()}, " ")
	}
	const want = "14 9 young 2026-05-20"
	if got := growth(e.plant(c, p.Id)); got != want {
		t.Errorf("get = %s, want %s", got, want)
	}
	list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	got := map[string]string{}
	for _, it := range list.Items {
		got[it.Name] = growth(it)
	}
	if got["Monstera"] != want || got["Fern"] != "none" {
		t.Errorf("list = %v", got)
	}
	e.newTask(c, p.Id, `{"kind":"check_in","interval_days":30}`)
	if due := e.today(c).Due; len(due) != 1 || growth(due[0].Plant) != want {
		t.Errorf("today = %+v", due)
	}
	if got := growth(e.plant(c, bare.Id)); got != "none" {
		t.Errorf("bare plant = %s", got)
	}
}
