package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const testID = "01JABCDEFGHJKMNPQRSTVWXYZ0"

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func size(t *testing.T, path string) image.Point {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return image.Pt(cfg.Width, cfg.Height)
}

func TestSave(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		want          Image
		original      string
		small, medium image.Point
	}{
		{"large png", testPNG(t, 2000, 1000), Image{"image/png", 2000, 1000}, "original.png", image.Pt(256, 128), image.Pt(1024, 512)},
		{"tiny png is not enlarged", testPNG(t, 100, 50), Image{"image/png", 100, 50}, "original.png", image.Pt(100, 50), image.Pt(100, 50)},
		{"portrait jpeg", testJPEG(t, 600, 1200), Image{"image/jpeg", 600, 1200}, "original.jpg", image.Pt(128, 256), image.Pt(512, 1024)},
		{"rotated jpeg", exifJPEG(t, 300, 200, binary.LittleEndian, 6), Image{"image/jpeg", 200, 300}, "original.jpg", image.Pt(171, 256), image.Pt(200, 300)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			got, err := s.Save(testID, tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Save = %+v, want %+v", got, tt.want)
			}
			dir := filepath.Join(s.dir, "photos", testID)
			if b, err := os.ReadFile(filepath.Join(dir, tt.original)); err != nil || !bytes.Equal(b, tt.data) {
				t.Errorf("original not stored byte for byte (err %v)", err)
			}
			if got := size(t, filepath.Join(dir, "small.jpg")); got != tt.small {
				t.Errorf("small = %v, want %v", got, tt.small)
			}
			if got := size(t, filepath.Join(dir, "medium.jpg")); got != tt.medium {
				t.Errorf("medium = %v, want %v", got, tt.medium)
			}
			if leftovers, _ := filepath.Glob(filepath.Join(s.dir, ".upload-*")); len(leftovers) != 0 {
				t.Errorf("temp dirs left behind: %v", leftovers)
			}
		})
	}
}

func TestSaveRejects(t *testing.T) {
	// A JPEG whose header claims 10000×10000 pixels: refused before decoding.
	bomb := testJPEG(t, 8, 8)
	sof := bytes.Index(bomb, []byte{0xFF, 0xC0})
	binary.BigEndian.PutUint16(bomb[sof+5:], 10000)
	binary.BigEndian.PutUint16(bomb[sof+7:], 10000)

	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"text", []byte("not an image at all"), ErrUnsupported},
		{"gif", gif, ErrUnsupported},
		{"empty", nil, ErrUnsupported},
		{"corrupt png", append([]byte("\x89PNG\r\n\x1a\n"), "garbage"...), ErrUnreadable},
		{"truncated jpeg", testJPEG(t, 64, 64)[:200], ErrUnreadable},
		{"decompression bomb", bomb, ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			if _, err := s.Save(testID, tt.data); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if _, err := os.Stat(filepath.Join(s.dir, "photos", testID)); !os.IsNotExist(err) {
				t.Errorf("photo dir exists after a rejected save")
			}
		})
	}
}

func TestOpenAndRemove(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Save(testID, testPNG(t, 10, 10)); err != nil {
		t.Fatal(err)
	}
	for size, want := range map[string]string{"small": "small.jpg", "medium": "medium.jpg", "original": "original.png"} {
		f, name, err := s.Open(testID, size)
		if err != nil || name != want {
			t.Fatalf("Open(%s) = %q, %v", size, name, err)
		}
		f.Close()
	}
	for _, tt := range []struct{ id, size string }{
		{testID, "large"},
		{testID, "../" + testID + "/small"},
		{"01JABCDEFGHJKMNPQRSTVWXYZ1", "small"}, // unknown
		{"01jabcdefghjkmnpqrstvwxyz0", "small"}, // lower case
		{"../../../../../../etc/passwd", "small"},
		{"01JABCDEFGHJKMNPQRSTVWXYZI", "small"}, // I is not Crockford base32
	} {
		if _, _, err := s.Open(tt.id, tt.size); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q, %q) err = %v, want ErrNotFound", tt.id, tt.size, err)
		}
	}

	if err := s.Remove(testID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(testID, "small"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after Remove: err = %v", err)
	}
	if err := s.Remove(testID); err != nil {
		t.Errorf("second Remove: %v", err)
	}
	if err := s.Remove("../escape"); err == nil {
		t.Error("Remove accepted an invalid id")
	}
}

func TestCleanTemp(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	leftover := filepath.Join(dir, "media", ".upload-123")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(dir, "media", "photos")
	if err := os.MkdirAll(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanTemp(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("leftover upload dir still exists: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("photos dir removed: %v", err)
	}
}
