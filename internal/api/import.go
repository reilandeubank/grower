package api

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/media"
	"github.com/reilandeubank/grower/internal/push"
)

// ErrNotEmpty means the instance already has user content, which an import never mixes with.
var ErrNotEmpty = errors.New("this grower already has data; import only works into an empty one")

// Limits on what an import reads from the zip, so a crafted file can't exhaust memory.
const (
	maxExportJSON = 256 << 20
	maxPhotoFile  = MaxUploadBytes
)

// ImportSummary counts the rows an import added.
type ImportSummary struct {
	Plants, Locations, Tasks, Events, Photos, Measurements, Species int
}

func (s ImportSummary) String() string {
	return fmt.Sprintf("Imported %d plants, %d locations, %d tasks, %d events, %d photos, %d measurements, %d species.",
		s.Plants, s.Locations, s.Tasks, s.Events, s.Photos, s.Measurements, s.Species)
}

// Import loads an export zip (docs/export-format.md) into an empty grower for one user:
// the only user, or username when there are several. It validates everything first,
// then writes the photos, then inserts every row in one transaction, keeping ids and
// timestamps. If the transaction fails, the photos it wrote are removed again, so a
// failed import leaves nothing behind.
func Import(ctx context.Context, d *db.DB, store *media.Store, zipPath, username string, now time.Time) (ImportSummary, error) {
	userID, err := importUser(ctx, d.Reader(), username)
	if err != nil {
		return ImportSummary{}, err
	}
	if n, err := d.Reader().CountContent(ctx); err != nil {
		return ImportSummary{}, fmt.Errorf("count existing data: %w", err)
	} else if n > 0 {
		return ImportSummary{}, ErrNotEmpty
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("open %s: %w", zipPath, err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		if _, dup := files[f.Name]; !dup {
			files[f.Name] = f
		}
	}
	x, err := readExport(files)
	if err != nil {
		return ImportSummary{}, err
	}
	if err := validateExport(x, files); err != nil {
		return ImportSummary{}, err
	}

	images, err := savePhotos(store, x.Photos, files)
	if err != nil {
		return ImportSummary{}, err
	}
	if err := writeExport(ctx, d, x, images, userID, now); err != nil {
		removePhotos(store, images)
		return ImportSummary{}, err
	}
	return ImportSummary{
		Plants: len(x.Plants), Locations: len(x.Locations), Tasks: len(x.Tasks), Events: len(x.Events),
		Photos: len(x.Photos), Measurements: len(x.Measurements), Species: len(x.Species),
	}, nil
}

// importUser picks the user the settings go to.
func importUser(ctx context.Context, q *dbgen.Queries, username string) (string, error) {
	users, err := q.ListUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("list users: %w", err)
	}
	if username != "" {
		for _, u := range users {
			if u.Username == username {
				return u.ID, nil
			}
		}
		return "", fmt.Errorf("no user named %q", username)
	}
	switch len(users) {
	case 0:
		return "", errors.New("no user yet; create one first with: grower user create --username NAME")
	case 1:
		return users[0].ID, nil
	}
	return "", errors.New("there are several users; choose one with --username")
}

// UnmarshalJSON gives a task from an export older than moisture checks the schema's
// default water_at; the other later fields default to their zero values.
func (t *exportTask) UnmarshalJSON(b []byte) error {
	type plain exportTask
	p := plain{WaterAt: 3}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*t = exportTask(p)
	return nil
}

func readExport(files map[string]*zip.File) (exportFile, error) {
	var x exportFile
	f, ok := files["export.json"]
	if !ok {
		return x, errors.New("export.json is not in the zip; is this a grower export?")
	}
	b, err := readZipFile(f, maxExportJSON)
	if err != nil {
		return x, err
	}
	// The header first, so a newer version is reported as such rather than as bad JSON.
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(b, &head); err != nil || head.Format != "grower-export" {
		return x, errors.New("export.json is not a grower export")
	}
	if head.Version != 1 {
		return x, fmt.Errorf("export version %d is not supported; this grower reads version 1", head.Version)
	}
	if err := json.Unmarshal(b, &x); err != nil {
		return x, fmt.Errorf("export.json: %w", err)
	}
	return x, nil
}

// readZipFile reads one entry, refusing more than limit bytes.
func readZipFile(f *zip.File, limit int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s in the zip: %w", f.Name, err)
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s in the zip: %w", f.Name, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s in the zip is larger than %d MB", f.Name, limit>>20)
	}
	return b, nil
}

// validateExport checks everything the database can't: ids, every reference, the
// settings, and that each photo file is present and intact.
func validateExport(x exportFile, files map[string]*zip.File) error {
	var errs []error
	ids := func(kind string, n int, id func(int) string) map[string]bool {
		seen := make(map[string]bool, n)
		for i := range n {
			s := id(i)
			if u, err := ulid.ParseStrict(s); err != nil || u.String() != s {
				errs = append(errs, fmt.Errorf("%s id %q is not a grower id", kind, s))
			} else if seen[s] {
				errs = append(errs, fmt.Errorf("%s %s is listed twice", kind, s))
			}
			seen[s] = true
		}
		return seen
	}
	locations := ids("location", len(x.Locations), func(i int) string { return x.Locations[i].ID })
	species := ids("species", len(x.Species), func(i int) string { return x.Species[i].ID })
	plants := ids("plant", len(x.Plants), func(i int) string { return x.Plants[i].ID })
	tasks := ids("task", len(x.Tasks), func(i int) string { return x.Tasks[i].ID })
	ids("event", len(x.Events), func(i int) string { return x.Events[i].Id })
	ids("measurement", len(x.Measurements), func(i int) string { return x.Measurements[i].ID })
	photoPlant := make(map[string]string, len(x.Photos))
	for _, p := range x.Photos {
		photoPlant[p.ID] = p.PlantID
	}
	ids("photo", len(x.Photos), func(i int) string { return x.Photos[i].ID })

	missing := func(what, id, ref string, refs map[string]bool) {
		if !refs[ref] {
			errs = append(errs, fmt.Errorf("%s %s refers to %s, which is not in the export", what, id, ref))
		}
	}
	ownPhoto := func(what, id, plantID, photoID string) {
		if owner, ok := photoPlant[photoID]; !ok {
			errs = append(errs, fmt.Errorf("%s %s refers to photo %s, which is not in the export", what, id, photoID))
		} else if owner != plantID {
			errs = append(errs, fmt.Errorf("%s %s refers to photo %s of another plant", what, id, photoID))
		}
	}
	for _, p := range x.Plants {
		if p.LocationID != nil {
			missing("plant", p.ID, *p.LocationID, locations)
		}
		if p.SpeciesID != nil {
			missing("plant", p.ID, *p.SpeciesID, species)
		}
		if p.CoverPhotoID != nil {
			ownPhoto("plant", p.ID, p.ID, *p.CoverPhotoID)
		}
	}
	for _, t := range x.Tasks {
		missing("task", t.ID, t.PlantID, plants)
	}
	for _, e := range x.Events {
		missing("event", e.Id, e.TaskId, tasks)
	}
	for _, m := range x.Measurements {
		missing("measurement", m.ID, m.PlantID, plants)
		if m.PhotoID != nil {
			ownPhoto("measurement", m.ID, m.PlantID, *m.PhotoID)
		}
	}
	for _, p := range x.Photos {
		missing("photo", p.ID, p.PlantID, plants)
		if err := checkPhotoFile(p, files); err != nil {
			errs = append(errs, err)
		}
	}
	errs = append(errs, validateSettings(x.Settings))
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("the export is not valid; nothing was imported:\n%w", err)
	}
	return nil
}

func checkPhotoFile(p exportPhoto, files map[string]*zip.File) error {
	f, ok := files[p.File]
	if !ok {
		return fmt.Errorf("photo %s: %s is not in the zip (the export itself lacked it: exports skip photo files missing on the server)", p.ID, p.File)
	}
	b, err := readZipFile(f, maxPhotoFile)
	if err != nil {
		return fmt.Errorf("photo %s: %w", p.ID, err)
	}
	if sum := sha256.Sum256(b); hex.EncodeToString(sum[:]) != p.Sha256 {
		return fmt.Errorf("photo %s: %s does not match its sha256; the zip is damaged", p.ID, p.File)
	}
	return nil
}

// validateSettings accepts blank fields (older exports), which keep the user's current values.
func validateSettings(s apigen.Settings) error {
	var errs []error
	if tz := s.Timezone; tz != "" {
		if _, err := time.LoadLocation(tz); err != nil || tz == "Local" {
			errs = append(errs, fmt.Errorf("settings: unknown time zone %q", tz))
		}
	}
	if s.DigestTime != "" {
		if _, err := push.ParseClock(s.DigestTime); err != nil {
			errs = append(errs, fmt.Errorf("settings: %w", err))
		}
	}
	if s.LaterTime != "" {
		if _, err := push.ParseClock(s.LaterTime); err != nil {
			errs = append(errs, fmt.Errorf("settings: later time: %w", err))
		}
	}
	if s.Hemisphere != "" && !s.Hemisphere.Valid() {
		errs = append(errs, fmt.Errorf("settings: hemisphere %q is not north or south", s.Hemisphere))
	}
	return errors.Join(errs...)
}

// savedPhoto is what the media store found in a photo's file.
type savedPhoto struct {
	image media.Image
	bytes int
}

// savePhotos writes every photo's original and thumbnails. On failure it removes the
// ones it wrote.
func savePhotos(store *media.Store, photos []exportPhoto, files map[string]*zip.File) (map[string]savedPhoto, error) {
	saved := make(map[string]savedPhoto, len(photos))
	for _, p := range photos {
		// Read again rather than held from validation, so only one photo is in memory.
		b, err := readZipFile(files[p.File], maxPhotoFile)
		var img media.Image
		if err == nil {
			img, err = store.Save(p.ID, b)
		}
		if err != nil {
			removePhotos(store, saved)
			return nil, fmt.Errorf("photo %s: %w", p.ID, err)
		}
		saved[p.ID] = savedPhoto{image: img, bytes: len(b)}
	}
	return saved, nil
}

func removePhotos(store *media.Store, saved map[string]savedPhoto) {
	for id := range saved {
		_ = store.Remove(id) // best effort: the rows were never committed
	}
}

// createdAt is when a row without an exported timestamp was made: a ULID carries it.
func createdAt(id string) string {
	return db.FormatTime(ulid.Time(ulid.MustParse(id).Time()))
}
