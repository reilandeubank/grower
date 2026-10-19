package server

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

// exported mirrors docs/export-format.md, decoded loosely so the test checks the
// field names as written, not our Go types.
type exported struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	ExportedAt string           `json:"exported_at"`
	AppVersion string           `json:"app_version"`
	Settings   map[string]any   `json:"settings"`
	Locations  []map[string]any `json:"locations"`
	Plants     []map[string]any `json:"plants"`
	Tasks      []map[string]any `json:"tasks"`
	Events     []map[string]any `json:"events"`
	Photos     []map[string]any `json:"photos"`
}

func TestExport(t *testing.T) {
	e := newEnv(t, envOptions{})
	e.clock.t = time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC) // April 30 in Chicago
	c := e.loginCookie()

	loc := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Shelf","light":"medium"}`), http.StatusCreated)
	monty := expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants",
		`{"name":"Monty","location_id":"`+loc.Id+`","acquired_on":"2026-04-12","pot_type":"terracotta","pot_size_in":6.5,"drainage":true,"soil":"Aroid mix"}`),
		http.StatusCreated)
	old := e.newPlant(c, "Old fern")
	expectJSON[apigen.Plant](t, e.call(c, http.MethodPut, "/api/v1/plants/"+old.Id, `{"name":"Old fern","archived":true}`), http.StatusOK)
	task := e.newTask(c, monty.Id, `{"kind":"water","interval_days":7}`)
	done := e.logEvent(c, task.Id, `{"kind":"done"}`).Event
	undone := e.logEvent(c, task.Id, `{"kind":"skipped"}`).Event
	e.call(c, http.MethodDelete, "/api/v1/events/"+undone.Id, "")
	png := testPNG(t, 300, 200, 1)
	cover := e.newPhoto(c, monty.Id, png, "2026-04-01T10:00:00Z")
	missing := e.newPhoto(c, monty.Id, testPNG(t, 300, 200, 2), "2026-04-02T10:00:00Z")
	if err := os.RemoveAll(e.photoDir(missing.Id)); err != nil {
		t.Fatal(err)
	}

	resp := e.call(c, http.MethodGet, "/api/v1/export", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for k, want := range map[string]string{
		"Content-Type":        "application/zip",
		"Content-Disposition": `attachment; filename="grower-export-2026-04-30.zip"`,
		"Cache-Control":       "no-store",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	files := unzip(t, body)

	raw := files["export.json"]
	for _, secret := range []string{"argon2id", "password", testPassword} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("export.json contains %q", secret)
		}
	}
	var x exported
	if err := json.Unmarshal(raw, &x); err != nil {
		t.Fatal(err)
	}
	if x.Format != "grower-export" || x.Version != 1 || x.ExportedAt != "2026-05-01T03:00:00Z" || x.AppVersion == "" ||
		x.Settings["timezone"] != "America/Chicago" || x.Settings["digest_time"] != "08:00" {
		t.Errorf("header = %+v", x)
	}
	if len(x.Locations) != 1 || len(x.Plants) != 2 || len(x.Tasks) != 1 || len(x.Events) != 2 || len(x.Photos) != 2 {
		t.Fatalf("counts: %d locations, %d plants, %d tasks, %d events, %d photos",
			len(x.Locations), len(x.Plants), len(x.Tasks), len(x.Events), len(x.Photos))
	}

	wantKeys(t, "location", x.Locations[0], "id", "name", "light", "notes")
	if l := x.Locations[0]; l["light"] != "medium" {
		t.Errorf("location = %v", l)
	}
	byID := func(items []map[string]any, id string) map[string]any {
		for _, it := range items {
			if it["id"] == id {
				return it
			}
		}
		t.Fatalf("id %s not exported", id)
		return nil
	}
	p := byID(x.Plants, monty.Id)
	wantKeys(t, "plant", p, "id", "name", "species", "location_id", "acquired_on", "notes", "pot_type", "pot_size_in",
		"drainage", "soil", "cover_photo_id", "archived_at", "created_at")
	if p["location_id"] != loc.Id || p["acquired_on"] != "2026-04-12" || p["pot_type"] != "terracotta" || p["pot_size_in"] != 6.5 ||
		p["drainage"] != true || p["soil"] != "Aroid mix" || p["cover_photo_id"] != cover.Id || p["archived_at"] != nil {
		t.Errorf("plant = %v", p)
	}
	if a := byID(x.Plants, old.Id); a["archived_at"] != "2026-05-01T03:00:00Z" || a["pot_type"] != nil {
		t.Errorf("archived plant = %v", a)
	}
	wantKeys(t, "task", x.Tasks[0], "id", "plant_id", "kind", "label", "interval_days", "anchor", "start_on", "seasonal", "moisture_check", "water_at")
	if t0 := x.Tasks[0]; t0["seasonal"] != true || t0["moisture_check"] != false || t0["water_at"] != 3.0 || x.Settings["hemisphere"] != "north" {
		t.Errorf("task = %v, settings = %v", x.Tasks[0], x.Settings)
	}
	if ev := byID(x.Events, done.Id); ev["voided_at"] != nil || ev["kind"] != "done" || ev["local_date"] != "2026-04-30" {
		t.Errorf("done event = %v", ev)
	}
	ev := byID(x.Events, undone.Id)
	wantKeys(t, "event", ev, "id", "task_id", "kind", "local_date", "snooze_until", "reading", "note", "occurred_at", "voided_at")
	if ev["voided_at"] != "2026-05-01T03:00:00Z" {
		t.Errorf("voided event = %v", ev)
	}

	ph := byID(x.Photos, cover.Id)
	wantKeys(t, "photo", ph, "id", "plant_id", "caption", "taken_at", "width", "height", "file", "sha256")
	sum := sha256.Sum256(png)
	if ph["file"] != "photos/"+cover.Id+".png" || ph["sha256"] != hex.EncodeToString(sum[:]) || ph["taken_at"] != "2026-04-01T10:00:00Z" {
		t.Errorf("photo = %v", ph)
	}
	if !bytes.Equal(files["photos/"+cover.Id+".png"], png) {
		t.Error("photo bytes differ from the original")
	}
	// The missing file is skipped, but its row is still listed.
	if _, ok := files["photos/"+missing.Id+".png"]; ok || byID(x.Photos, missing.Id)["file"] != "photos/"+missing.Id+".png" {
		t.Error("missing photo handled wrongly")
	}
	if len(files) != 2 {
		t.Errorf("zip holds %d files", len(files))
	}
	expectProblem(t, e.call(nil, http.MethodGet, "/api/v1/export", ""), http.StatusUnauthorized)
}

// unzip returns every file in a zip by name, checking that photos are stored as is.
func unzip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "photos/") && f.Method != zip.Store {
			t.Errorf("%s: method %d, want store", f.Name, f.Method)
		}
		if filepath.IsAbs(f.Name) || strings.Contains(f.Name, "..") {
			t.Errorf("unsafe name %q", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = b
	}
	return files
}

// wantKeys checks an exported object has exactly these keys.
func wantKeys(t *testing.T, what string, obj map[string]any, keys ...string) {
	t.Helper()
	if len(obj) != len(keys) {
		t.Errorf("%s has %d keys, want %d: %v", what, len(obj), len(keys), obj)
	}
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			t.Errorf("%s lacks %q: %v", what, k, obj)
		}
	}
}
