package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
)

// testPNG returns a w×h PNG; shade makes otherwise equal images differ in bytes.
func testPNG(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.SetRGBA(0, 0, color.RGBA{R: shade, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// multipartBody builds an upload body; file is omitted when nil.
func multipartBody(t *testing.T, file []byte, fields map[string]string) (body, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if file != nil {
		fw, err := mw.CreateFormFile("file", "photo.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(file)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), mw.FormDataContentType()
}

func (e *env) upload(c *http.Cookie, plantID string, file []byte, fields map[string]string) *http.Response {
	e.t.Helper()
	body, ct := multipartBody(e.t, file, fields)
	return e.do(request{method: http.MethodPost, path: "/api/v1/plants/" + plantID + "/photos", body: body,
		cookie: c, headers: map[string]string{"Content-Type": ct}})
}

func (e *env) newPhoto(c *http.Cookie, plantID string, file []byte, takenAt string) apigen.Photo {
	e.t.Helper()
	return expectJSON[apigen.Photo](e.t, e.upload(c, plantID, file, map[string]string{"taken_at": takenAt}), http.StatusCreated)
}

func (e *env) photoDir(id string) string { return filepath.Join(e.cfg.DataDir, "media", "photos", id) }

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestPhotoUpload(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monty")
	e.newTask(c, p.Id, `{"kind":"water","interval_days":7}`)
	first := testPNG(t, 2000, 1000, 1)

	got := expectJSON[apigen.Photo](t, e.upload(c, p.Id, first, map[string]string{
		"caption": " New leaf ", "taken_at": "2026-04-01T10:00:00-05:00",
	}), http.StatusCreated)
	if got.PlantId != p.Id || got.Caption != "New leaf" || got.Width != 2000 || got.Height != 1000 ||
		got.TakenAt.Format("2006-01-02T15:04:05Z07:00") != "2026-04-01T15:00:00Z" ||
		got.Urls.Small != "/media/photos/"+got.Id+"/small" || got.Urls.Medium != "/media/photos/"+got.Id+"/medium" ||
		got.Urls.Original != "/media/photos/"+got.Id+"/original" {
		t.Errorf("photo = %+v", got)
	}
	for _, name := range []string{"original.png", "small.jpg", "medium.jpg"} {
		if !exists(filepath.Join(e.photoDir(got.Id), name)) {
			t.Errorf("%s missing", name)
		}
	}

	// The first photo becomes the cover, everywhere a plant appears.
	if pl := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id, ""), http.StatusOK); pl.Cover == nil || *pl.Cover != got {
		t.Errorf("cover = %+v, want %+v", pl.Cover, got)
	}
	if list := expectJSON[apigen.PlantList](t, e.call(c, http.MethodGet, "/api/v1/plants", ""), http.StatusOK); list.Items[0].Cover == nil || list.Items[0].Cover.Id != got.Id {
		t.Errorf("list cover = %+v", list.Items[0].Cover)
	}
	if today := e.today(c); len(today.Due) != 1 || today.Due[0].Plant.Cover == nil || today.Due[0].Plant.Cover.Id != got.Id {
		t.Errorf("today = %+v", today)
	}

	// A second photo, taken later by default (now), does not take over the cover.
	second := expectJSON[apigen.Photo](t, e.upload(c, p.Id, testPNG(t, 10, 10, 2), nil), http.StatusCreated)
	if !second.TakenAt.Equal(e.clock.t) {
		t.Errorf("default taken_at = %v, want now", second.TakenAt)
	}
	if pl := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id, ""), http.StatusOK); pl.Cover.Id != got.Id {
		t.Errorf("second upload changed the cover to %s", pl.Cover.Id)
	}

	// The same bytes again are a no-op that returns the existing photo.
	if dup := expectJSON[apigen.Photo](t, e.upload(c, p.Id, first, nil), http.StatusOK); dup.Id != got.Id {
		t.Errorf("duplicate id = %s, want %s", dup.Id, got.Id)
	}
	// The same bytes on another plant are a new photo.
	other := e.newPlant(c, "Aloe")
	if o := e.newPhoto(c, other.Id, first, "2026-04-01T00:00:00Z"); o.Id == got.Id {
		t.Error("photo shared across plants")
	}

	list := expectJSON[apigen.PhotoList](t, e.call(c, http.MethodGet, "/api/v1/plants/"+p.Id+"/photos", ""), http.StatusOK)
	if len(list.Items) != 2 || list.Items[0].Id != second.Id || list.Items[1].Id != got.Id {
		t.Errorf("list not newest first: %+v", list.Items)
	}
	expectProblem(t, e.call(c, http.MethodGet, "/api/v1/plants/nope/photos", ""), http.StatusNotFound)
}

func TestPhotoUploadValidation(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monty")
	img := testPNG(t, 4, 4, 0)
	tests := []struct {
		name   string
		plant  string
		file   []byte
		fields map[string]string
		status int
		detail string
	}{
		{"not an image", p.Id, []byte("just some text"), nil, 400, "Only JPEG and PNG photos are supported."},
		{"gif", p.Id, []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), nil, 400, "Only JPEG and PNG photos are supported."},
		{"corrupt png", p.Id, append([]byte("\x89PNG\r\n\x1a\n"), "junk"...), nil, 400, "That photo could not be read. It may be damaged."},
		{"no file", p.Id, nil, map[string]string{"caption": "hi"}, 400, "Choose a photo to upload."},
		{"long caption", p.Id, img, map[string]string{"caption": strings.Repeat("a", 501)}, 400, "Caption must be 500 characters or fewer."},
		{"bad date", p.Id, img, map[string]string{"taken_at": "yesterday"}, 400, "Taken at must be an RFC 3339 date and time."},
		{"unknown plant", "nope", img, nil, 404, "Plant not found."},
		{"over 15 MB", p.Id, make([]byte, 15<<20), nil, 413, "Photos must be 15 MB or smaller."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectProblem(t, e.upload(c, tt.plant, tt.file, tt.fields), tt.status); got != tt.detail {
				t.Errorf("detail = %q, want %q", got, tt.detail)
			}
		})
	}
	if entries, _ := os.ReadDir(filepath.Join(e.cfg.DataDir, "media", "photos")); len(entries) != 0 {
		t.Errorf("rejected uploads left files: %v", entries)
	}
}

func TestPhotoEditDeleteAndCover(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monty")
	plantPath := "/api/v1/plants/" + p.Id
	cover := func() string {
		t.Helper()
		pl := expectJSON[apigen.Plant](t, e.call(c, http.MethodGet, plantPath, ""), http.StatusOK)
		if pl.Cover == nil {
			return ""
		}
		return pl.Cover.Id
	}
	setCover := func(body string) *http.Response { return e.call(c, http.MethodPut, plantPath+"/cover", body) }

	p1 := e.newPhoto(c, p.Id, testPNG(t, 4, 4, 1), "2026-04-01T00:00:00Z")
	p2 := e.newPhoto(c, p.Id, testPNG(t, 4, 4, 2), "2026-04-02T00:00:00Z")
	p3 := e.newPhoto(c, p.Id, testPNG(t, 4, 4, 3), "2026-04-03T00:00:00Z")
	if cover() != p1.Id {
		t.Fatalf("cover = %s, want first upload", cover())
	}

	// Update
	upd := expectJSON[apigen.Photo](t, e.call(c, http.MethodPut, "/api/v1/photos/"+p1.Id,
		`{"caption":"Day one","taken_at":"2026-03-01T08:00:00Z"}`), http.StatusOK)
	if upd.Caption != "Day one" || upd.TakenAt.Format("2006-01-02") != "2026-03-01" || upd.Width != 4 {
		t.Errorf("updated = %+v", upd)
	}
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/photos/"+p1.Id, `{"caption":"x"}`), http.StatusBadRequest)
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/photos/"+p1.Id,
		`{"caption":"`+strings.Repeat("a", 501)+`","taken_at":"2026-03-01T08:00:00Z"}`), http.StatusBadRequest)
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/photos/nope", `{"caption":"","taken_at":"2026-03-01T08:00:00Z"}`), http.StatusNotFound)

	// Set cover
	other := e.newPlant(c, "Aloe")
	foreign := e.newPhoto(c, other.Id, testPNG(t, 4, 4, 9), "2026-04-01T00:00:00Z")
	for _, body := range []string{`{"photo_id":"` + foreign.Id + `"}`, `{"photo_id":"nope"}`} {
		if d := expectProblem(t, setCover(body), http.StatusBadRequest); d != "That photo belongs to another plant." {
			t.Errorf("detail = %q", d)
		}
	}
	expectProblem(t, e.call(c, http.MethodPut, "/api/v1/plants/nope/cover", `{"photo_id":null}`), http.StatusNotFound)
	if pl := expectJSON[apigen.Plant](t, setCover(`{"photo_id":"`+p2.Id+`"}`), http.StatusOK); pl.Cover == nil || pl.Cover.Id != p2.Id {
		t.Errorf("set cover = %+v", pl.Cover)
	}

	// Deleting the cover hands it to the newest remaining photo, and removes the files.
	if resp := e.call(c, http.MethodDelete, "/api/v1/photos/"+p2.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	if exists(e.photoDir(p2.Id)) {
		t.Error("deleted photo's files remain")
	}
	if cover() != p3.Id {
		t.Errorf("cover after delete = %s, want newest %s", cover(), p3.Id)
	}
	expectProblem(t, e.call(c, http.MethodDelete, "/api/v1/photos/"+p2.Id, ""), http.StatusNotFound)

	// A cleared cover stays cleared when another photo is deleted.
	if pl := expectJSON[apigen.Plant](t, setCover(`{"photo_id":null}`), http.StatusOK); pl.Cover != nil {
		t.Errorf("cleared cover = %+v", pl.Cover)
	}
	e.call(c, http.MethodDelete, "/api/v1/photos/"+p3.Id, "")
	if cover() != "" {
		t.Errorf("cover = %s, want none", cover())
	}

	// Deleting the plant removes its remaining photos' files, and only those.
	if !exists(e.photoDir(p1.Id)) {
		t.Fatal("p1 files missing before plant delete")
	}
	if resp := e.call(c, http.MethodDelete, plantPath, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete plant status = %d", resp.StatusCode)
	}
	if exists(e.photoDir(p1.Id)) {
		t.Error("plant delete left photo files")
	}
	if !exists(e.photoDir(foreign.Id)) {
		t.Error("plant delete removed another plant's photo")
	}
}

func TestPhotoRoutesNeedAuth(t *testing.T) {
	e := newEnv(t, envOptions{})
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/plants/x/photos"},
		{http.MethodPut, "/api/v1/plants/x/cover"},
		{http.MethodPut, "/api/v1/photos/x"},
		{http.MethodDelete, "/api/v1/photos/x"},
	} {
		expectProblem(t, e.call(nil, r.method, r.path, `{}`), http.StatusUnauthorized)
	}
	expectProblem(t, e.upload(nil, "x", testPNG(t, 1, 1, 0), nil), http.StatusUnauthorized)
}
