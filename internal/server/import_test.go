package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/media"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.SetRGBA(1, 1, color.RGBA{G: 200, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// richExport builds every kind of data through the API and returns its export zip.
func richExport(t *testing.T) []byte {
	t.Helper()
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	expectJSON[apigen.Settings](t, e.call(c, http.MethodPut, "/api/v1/settings",
		`{"timezone":"Australia/Sydney","digest_time":"07:30","later_time":"18:45","hemisphere":"south"}`), http.StatusOK)

	shelf := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations",
		`{"name":"Shelf","light":"medium","notes":"By the desk","windows":[{"direction":"w","count":2},{"direction":"ne","count":1}]}`), http.StatusCreated)
	expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Hall"}`), http.StatusCreated)
	sp := e.newSpecies(c, `{"scientific_name":"Monstera deliciosa","common_name":"Swiss cheese plant","water_interval_days":7,"light":"bright_indirect","water_at":3,"notes":"Aroid"}`)
	e.newSpecies(c, `{"scientific_name":"Aloe vera"}`)

	monty := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants",
		`{"name":"Monty","species":"Monstera","species_id":"`+sp.Id+`","location_id":"`+shelf.Id+`","acquired_on":"2026-04-12","notes":"Gift",
		  "pot_type":"terracotta","pot_size_in":6.5,"drainage":true,"soil":"Aroid mix","light":"low","window_distance":"near","grow_light_hours":12}`),
		http.StatusCreated)
	fern := e.newPlant(c, "Fern")
	old := e.newPlant(c, "Old cactus")
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+old.Id, `{"name":"Old cactus","archived":true,"drainage":false}`), http.StatusOK)

	water := e.newTask(c, monty.Id, `{"kind":"water","interval_days":7,"moisture_check":true,"water_at":4}`)
	e.newTask(c, monty.Id, `{"kind":"fertilize","interval_days":30,"seasonal":false}`)
	custom := e.newTask(c, fern.Id, `{"kind":"custom","label":"Wipe leaves","interval_days":14,"anchor":"fixed","start_on":"2026-05-03"}`)
	checkIn := e.newTask(c, monty.Id, `{"kind":"check_in","interval_days":30}`)
	e.newTask(c, old.Id, `{"kind":"water","interval_days":10}`)

	e.logEvent(c, water.Id, `{"kind":"checked","reading":6,"note":"Still moist"}`)
	e.logEvent(c, water.Id, `{"kind":"done","reading":3}`)
	voided := e.logEvent(c, custom.Id, `{"kind":"skipped"}`).Event
	e.call(c, http.MethodDelete, "/api/v1/events/"+voided.Id, "")
	e.logEvent(c, custom.Id, `{"kind":"snoozed","snooze_until":"2026-05-05"}`)
	e.logEvent(c, checkIn.Id, `{"kind":"done","note":"Growing"}`)

	cover := e.newPhoto(c, monty.Id, testPNG(t, 300, 200, 1), "2026-04-01T10:00:00Z")
	e.upload(c, monty.Id, testJPEG(t, 640, 480), map[string]string{"taken_at": "2026-04-20T08:00:00Z", "caption": "New leaf"})
	fernPhoto := e.newPhoto(c, fern.Id, testPNG(t, 120, 300, 9), "2026-04-02T10:00:00Z")
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+fern.Id+"/cover", `{"photo_id":"`+fernPhoto.Id+`"}`), http.StatusOK)
	e.newMeasurement(c, monty.Id, `{"measured_on":"2026-04-12","height_in":20.5,"width_in":14,"stage":"mature","note":"Big","photo_id":"`+cover.Id+`"}`)
	e.newMeasurement(c, fern.Id, `{"measured_on":"2026-04-30","stage":"young"}`)

	resp := e.call(c, http.MethodGet, "/api/v1/export", "")
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("export: %d %v", resp.StatusCode, err)
	}
	return body
}

// importZip writes data to a file and imports it into e.
func importZip(e *env, data []byte, username string) (api.ImportSummary, error) {
	e.t.Helper()
	path := filepath.Join(e.t.TempDir(), "export.zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		e.t.Fatal(err)
	}
	return api.Import(context.Background(), e.db, media.New(e.cfg.DataDir), path, username, e.clock.Now())
}

// exportOf exports e and returns export.json without its per-run header fields, and
// every other file in the zip.
func exportOf(e *env, data []byte) (map[string]any, map[string][]byte) {
	e.t.Helper()
	if data == nil {
		resp := e.call(e.loginCookie(), http.MethodGet, "/api/v1/export", "")
		var err error
		if data, err = io.ReadAll(resp.Body); err != nil {
			e.t.Fatal(err)
		}
	}
	files := unzip(e.t, data)
	var x map[string]any
	if err := json.Unmarshal(files["export.json"], &x); err != nil {
		e.t.Fatal(err)
	}
	delete(x, "exported_at")
	delete(x, "app_version")
	delete(files, "export.json")
	return x, files
}

func TestImportRoundTrip(t *testing.T) {
	original := richExport(t)
	e := newEnv(t, envOptions{})
	got, err := importZip(e, original, "")
	if err != nil {
		t.Fatal(err)
	}
	if s := got.String(); s != "Imported 3 plants, 2 locations, 5 tasks, 5 events, 3 photos, 2 measurements, 2 species." {
		t.Errorf("summary = %q", s)
	}
	want, wantFiles := exportOf(e, original)
	var voided, readings int
	for _, ev := range want["events"].([]any) {
		if ev.(map[string]any)["voided_at"] != nil {
			voided++
		}
		if ev.(map[string]any)["reading"] != nil {
			readings++
		}
	}
	if voided != 1 || readings != 2 {
		t.Errorf("the dataset lacks voided events (%d) or readings (%d)", voided, readings)
	}
	again, againFiles := exportOf(e, nil)
	if !reflect.DeepEqual(want, again) {
		a, _ := json.MarshalIndent(want, "", " ")
		b, _ := json.MarshalIndent(again, "", " ")
		t.Errorf("re-export differs:\nwant %s\ngot  %s", a, b)
	}
	if len(wantFiles) != 3 || !reflect.DeepEqual(wantFiles, againFiles) {
		t.Errorf("photo files differ: %d vs %d", len(wantFiles), len(againFiles))
	}
	// Thumbnails are regenerated, and the derived fields work on the imported rows.
	for _, p := range want["photos"].([]any) {
		id := p.(map[string]any)["id"].(string)
		if !exists(filepath.Join(e.photoDir(id), "small.jpg")) || !exists(filepath.Join(e.photoDir(id), "medium.jpg")) {
			t.Errorf("photo %s has no thumbnails", id)
		}
	}
	plants := expectJSON[apigen.PlantList](t, e.call(e.loginCookie(), http.MethodGet, "/api/v1/plants", ""), http.StatusOK)
	if len(plants.Items) != 2 || plants.Items[0].Cover == nil {
		t.Errorf("active plants = %+v", plants.Items)
	}
}

// rewrite returns zip data with export.json changed by edit and the files in drop left out.
func rewrite(t *testing.T, data []byte, edit func(x map[string]any), drop ...string) []byte {
	t.Helper()
	files := unzip(t, data)
	var x map[string]any
	if err := json.Unmarshal(files["export.json"], &x); err != nil {
		t.Fatal(err)
	}
	edit(x)
	b, err := json.Marshal(x)
	if err != nil {
		t.Fatal(err)
	}
	files["export.json"] = b
	for _, name := range drop {
		delete(files, name)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func first(x map[string]any, key string) map[string]any { return x[key].([]any)[0].(map[string]any) }

func TestImportRejects(t *testing.T) {
	original := richExport(t)
	photoFile := func() string {
		var x map[string]any
		_ = json.Unmarshal(unzip(t, original)["export.json"], &x)
		return first(x, "photos")["file"].(string)
	}()
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"not a zip", []byte("hello"), "not a valid zip"},
		{"bad version", rewrite(t, original, func(x map[string]any) { x["version"] = 2 }), "export version 2 is not supported; this grower reads version 1"},
		{"bad format", rewrite(t, original, func(x map[string]any) { x["format"] = "other" }), "not a grower export"},
		{"broken reference", rewrite(t, original, func(x map[string]any) { first(x, "tasks")["plant_id"] = "01JAAAAAAAAAAAAAAAAAAAAAAA" }),
			"refers to 01JAAAAAAAAAAAAAAAAAAAAAAA, which is not in the export"},
		{"broken photo reference", rewrite(t, original, func(x map[string]any) { first(x, "measurements")["photo_id"] = "01JAAAAAAAAAAAAAAAAAAAAAAA" }),
			"refers to photo 01JAAAAAAAAAAAAAAAAAAAAAAA, which is not in the export"},
		{"sha mismatch", rewrite(t, original, func(x map[string]any) { first(x, "photos")["sha256"] = strings.Repeat("0", 64) }),
			"does not match its sha256"},
		{"missing photo file", rewrite(t, original, func(map[string]any) {}, photoFile),
			photoFile + " is not in the zip (the export itself lacked it"},
		{"bad settings", rewrite(t, original, func(x map[string]any) { x["settings"].(map[string]any)["timezone"] = "Mars/Base" }), "unknown time zone"},
		{"bad later time", rewrite(t, original, func(x map[string]any) { x["settings"].(map[string]any)["later_time"] = "5pm" }), "later time"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, envOptions{})
			_, err := importZip(e, tc.data, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			assertEmpty(e)
		})
	}
}

// assertEmpty checks that nothing was imported: no rows, no photo files, settings as before.
func assertEmpty(e *env) {
	e.t.Helper()
	if n, err := e.db.Reader().CountContent(context.Background()); err != nil || n != 0 {
		e.t.Errorf("%d rows left (%v)", n, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.cfg.DataDir, "media", "photos")); len(entries) != 0 {
		e.t.Errorf("%d photo dirs left", len(entries))
	}
	s := expectJSON[apigen.Me](e.t, e.call(e.loginCookie(), http.MethodGet, "/api/v1/me", ""), http.StatusOK).Settings
	if s.Timezone != "America/Chicago" || s.Hemisphere != "north" {
		e.t.Errorf("settings changed: %+v", s)
	}
}

func TestImportIsAtomic(t *testing.T) {
	original := richExport(t)
	e := newEnv(t, envOptions{})
	// Measurements are inserted last, after the photos are on disk and every other row is in.
	if _, err := e.db.Write.Exec(`CREATE TRIGGER fail BEFORE INSERT ON plant_measurements BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := importZip(e, original, ""); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("err = %v", err)
	}
	assertEmpty(e)
}

func TestImportNeedsEmptyGrowerAndOneUser(t *testing.T) {
	original := richExport(t)
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	e.newPlant(c, "Already here")
	if _, err := importZip(e, original, ""); !errors.Is(err, api.ErrNotEmpty) ||
		err.Error() != "this grower already has data; import only works into an empty one" {
		t.Errorf("err = %v", err)
	}

	e = newEnv(t, envOptions{})
	q := e.db.Writer()
	if err := q.CreateUser(context.Background(), dbgen.CreateUserParams{ID: "01JBBBBBBBBBBBBBBBBBBBBBBB", Username: "ivy", PasswordHash: "x", CreatedAt: "2026-05-01T00:00:00Z", UpdatedAt: "2026-05-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateSettings(context.Background(), dbgen.CreateSettingsParams{UserID: "01JBBBBBBBBBBBBBBBBBBBBBBB", Timezone: "UTC", CreatedAt: "2026-05-01T00:00:00Z", UpdatedAt: "2026-05-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := importZip(e, original, ""); err == nil || !strings.Contains(err.Error(), "--username") {
		t.Errorf("several users: err = %v", err)
	}
	if _, err := importZip(e, original, "nobody"); err == nil || !strings.Contains(err.Error(), `no user named "nobody"`) {
		t.Errorf("unknown user: err = %v", err)
	}
	if _, err := importZip(e, original, testUser); err != nil {
		t.Fatal(err)
	}
	s := expectJSON[apigen.Me](t, e.call(e.loginCookie(), http.MethodGet, "/api/v1/me", ""), http.StatusOK).Settings
	if s.Timezone != "Australia/Sydney" || s.DigestTime != "07:30" || s.LaterTime != "18:45" || s.Hemisphere != "south" {
		t.Errorf("settings = %+v", s)
	}
}

// TestImportOldExport imports an export from before species, light, soil checks and the
// journal: the missing fields take their defaults.
func TestImportOldExport(t *testing.T) {
	const old = `{"format":"grower-export","version":1,"exported_at":"2026-01-02T03:04:05Z","app_version":"v1.0.0",
	"settings":{"timezone":"Europe/London","digest_time":"09:00"},
	"locations":[{"id":"01JC0000000000000000000001","name":"Kitchen","light":"direct","notes":""}],
	"plants":[{"id":"01JC0000000000000000000002","name":"Basil","species":"Basil","location_id":"01JC0000000000000000000001",
	  "acquired_on":null,"notes":"","pot_type":null,"pot_size_in":null,"drainage":null,"soil":"","cover_photo_id":null,
	  "archived_at":null,"created_at":"2025-12-01T10:00:00Z"}],
	"tasks":[{"id":"01JC0000000000000000000003","plant_id":"01JC0000000000000000000002","kind":"water","label":"",
	  "interval_days":3,"anchor":"from_completion","start_on":"2025-12-01"}],
	"events":[{"id":"01JC0000000000000000000004","task_id":"01JC0000000000000000000003","kind":"done","local_date":"2025-12-04",
	  "snooze_until":null,"note":"","occurred_at":"2025-12-04T08:00:00Z","voided_at":null,"some_future_field":true}],
	"photos":[]}`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("export.json")
	_, _ = w.Write([]byte(old))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, envOptions{})
	got, err := importZip(e, buf.Bytes(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got != (api.ImportSummary{Plants: 1, Locations: 1, Tasks: 1, Events: 1}) {
		t.Errorf("summary = %+v", got)
	}
	x, _ := exportOf(e, nil)
	task, loc, plant := first(x, "tasks"), first(x, "locations"), first(x, "plants")
	if task["seasonal"] != false || task["moisture_check"] != false || task["water_at"] != 3.0 {
		t.Errorf("task = %v", task)
	}
	if loc["windows"] == nil || len(loc["windows"].([]any)) != 0 || plant["species_id"] != nil || plant["light"] != nil ||
		plant["created_at"] != "2025-12-01T10:00:00Z" || first(x, "events")["reading"] != nil {
		t.Errorf("location = %v, plant = %v", loc, plant)
	}
	if s := x["settings"].(map[string]any); s["timezone"] != "Europe/London" || s["digest_time"] != "09:00" || s["later_time"] != "17:30" || s["hemisphere"] != "north" {
		t.Errorf("settings = %v", s)
	}
}
