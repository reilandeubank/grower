// Package media stores photo files under <data>/media/photos/<photo id>/: the
// original bytes untouched (browsers honour its EXIF orientation) plus small and
// medium JPEG thumbnails, which are rotated upright here.
package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
	"net/http"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Errors for images the store will not accept.
var (
	ErrUnsupported = errors.New("media: not a JPEG or PNG image")
	ErrUnreadable  = errors.New("media: image data is corrupt")
	ErrTooLarge    = errors.New("media: image has too many pixels")
)

// ErrNotFound is returned by Open for an unknown photo or size.
var ErrNotFound = errors.New("media: no such file")

// maxPixels guards against decompression bombs: a small file that decodes to an
// enormous bitmap. 50 MP is beyond any phone camera.
const maxPixels = 50_000_000

// Thumbnail sizes, by long side in pixels.
var thumbs = map[string]int{"small": 256, "medium": 1024}

// Image describes a stored photo. Width and height are as displayed (EXIF-oriented).
type Image struct {
	MIME          string
	Width, Height int
}

// Store keeps photo files on disk.
type Store struct{ dir string }

// New returns a store rooted at <dataDir>/media.
func New(dataDir string) *Store { return &Store{dir: filepath.Join(dataDir, "media")} }

// Save validates data as a JPEG or PNG and writes the original and its thumbnails for
// photo id. Files are written to a temporary directory and renamed into place, so a
// crash never leaves a partial photo.
func (s *Store) Save(id string, data []byte) (Image, error) {
	if !validID(id) {
		return Image{}, fmt.Errorf("media: invalid photo id %q", id)
	}
	img, src, err := decode(data)
	if err != nil {
		return Image{}, err
	}
	o := 1
	if img.MIME == "image/jpeg" {
		o = orientation(data)
	}
	if o >= 5 {
		img.Width, img.Height = img.Height, img.Width
	}

	photos := filepath.Join(s.dir, "photos")
	if err := os.MkdirAll(photos, 0o750); err != nil {
		return Image{}, fmt.Errorf("create media dir: %w", err)
	}
	tmp, err := os.MkdirTemp(s.dir, ".upload-")
	if err != nil {
		return Image{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp) // a no-op once renamed

	if err := writeFile(filepath.Join(tmp, originalName(img.MIME)), data); err != nil {
		return Image{}, err
	}
	// Medium is scaled from the original and small from medium: much faster, and
	// at these ratios no visible loss.
	medium := orient(scale(src, thumbs["medium"]), o)
	small := scale(medium, thumbs["small"])
	for name, m := range map[string]image.Image{"medium.jpg": medium, "small.jpg": small} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, m, &jpeg.Options{Quality: 80}); err != nil {
			return Image{}, fmt.Errorf("encode %s: %w", name, err)
		}
		if err := writeFile(filepath.Join(tmp, name), buf.Bytes()); err != nil {
			return Image{}, err
		}
	}
	if err := os.Rename(tmp, filepath.Join(photos, id)); err != nil {
		return Image{}, fmt.Errorf("move photo %s into place: %w", id, err)
	}
	return img, nil
}

// decode sniffs, bounds and decodes an image.
func decode(data []byte) (Image, image.Image, error) {
	img := Image{MIME: http.DetectContentType(data)}
	if img.MIME != "image/jpeg" && img.MIME != "image/png" {
		return img, nil, ErrUnsupported
	}
	// The header alone gives the dimensions, so a bomb is refused before decoding.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return img, nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return img, nil, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return img, nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	img.Width, img.Height = cfg.Width, cfg.Height
	return img, src, nil
}

// scale fits src within long×long pixels, never enlarging it. Catmull-Rom widens its
// kernel when shrinking, so large reductions stay free of aliasing, unlike the
// approximate bilinear filter. Transparent PNG areas become white, as JPEG has no alpha.
func scale(src image.Image, long int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if m := max(w, h); m > long {
		w, h = max(1, (w*long+m/2)/m), max(1, (h*long+m/2)/m)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func writeFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640) //nolint:gosec // path built from a validated id
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync() // durable before the rename publishes it
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func originalName(mime string) string {
	if mime == "image/png" {
		return "original.png"
	}
	return "original.jpg"
}

// CleanTemp removes upload directories left behind by a crash mid-Save. Call it at
// startup, before any upload can be in flight.
func (s *Store) CleanTemp() error {
	dirs, err := filepath.Glob(filepath.Join(s.dir, ".upload-*"))
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range dirs {
		errs = append(errs, os.RemoveAll(d))
	}
	return errors.Join(errs...)
}

// Remove deletes a photo's files. Removing a photo that has none is not an error.
func (s *Store) Remove(id string) error {
	if !validID(id) {
		return fmt.Errorf("media: invalid photo id %q", id)
	}
	return os.RemoveAll(filepath.Join(s.dir, "photos", id))
}

// Open returns one file of a photo (size is small, medium or original) and its name,
// whose extension gives the content type. Both inputs come from the URL, so they are
// checked strictly before touching the filesystem.
func (s *Store) Open(id, size string) (*os.File, string, error) {
	var names []string
	switch {
	case !validID(id):
	case size == "original":
		names = []string{"original.jpg", "original.png"}
	case thumbs[size] != 0:
		names = []string{size + ".jpg"}
	}
	for _, name := range names {
		f, err := os.Open(filepath.Join(s.dir, "photos", id, name)) //nolint:gosec // id and name validated above
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return f, name, err
	}
	return nil, "", ErrNotFound
}

// validID accepts only ULIDs: 26 characters of Crockford base32, upper case.
func validID(id string) bool {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	if len(id) != 26 {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}
