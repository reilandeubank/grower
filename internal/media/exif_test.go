package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withAPP1 inserts an APP1 segment right after the JPEG's SOI marker.
func withAPP1(jpg, payload []byte) []byte {
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

// exifTIFF is a TIFF header plus IFD0 holding a Make tag, then Orientation = o
// with the given entry type (3 is SHORT).
func exifTIFF(order binary.ByteOrder, o, typ uint16) []byte {
	tiff := make([]byte, 8+2+2*12+4)
	copy(tiff, "MM")
	if order == binary.LittleEndian {
		copy(tiff, "II")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 2)
	mk := tiff[10:]
	order.PutUint16(mk, 0x010F)
	order.PutUint16(mk[2:], 2)
	order.PutUint32(mk[4:], 4)
	copy(mk[8:], "Foo\x00")
	e := tiff[22:]
	order.PutUint16(e, 0x0112)
	order.PutUint16(e[2:], typ)
	order.PutUint32(e[4:], 1)
	order.PutUint16(e[8:], o)
	return append([]byte("Exif\x00\x00"), tiff...)
}

func exifJPEG(t *testing.T, w, h int, order binary.ByteOrder, o uint16) []byte {
	t.Helper()
	return withAPP1(testJPEG(t, w, h), exifTIFF(order, o, 3))
}

func TestOrientation(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for o := range uint16(8) {
			if got := orientation(exifJPEG(t, 4, 4, order, o+1)); got != int(o+1) {
				t.Errorf("%v orientation %d: got %d", order, o+1, got)
			}
		}
	}

	plain := testJPEG(t, 4, 4)
	badMagic := exifTIFF(binary.BigEndian, 6, 3)
	badMagic[9] = 43
	badOffset := exifTIFF(binary.LittleEndian, 6, 3)
	binary.LittleEndian.PutUint32(badOffset[10:], 1<<30)
	truncated := withAPP1(plain, exifTIFF(binary.BigEndian, 6, 3))
	binary.BigEndian.PutUint16(truncated[4:], 0xFFF0) // segment length past the end
	tests := []struct {
		name string
		data []byte
	}{
		{"no exif", plain},
		{"empty", nil},
		{"not a jpeg", []byte("hello, world")},
		{"xmp app1", withAPP1(plain, []byte("http://ns.adobe.com/xap/1.0/\x00<x/>"))},
		{"bad tiff magic", withAPP1(plain, badMagic)},
		{"ifd offset out of range", withAPP1(plain, badOffset)},
		{"value out of range", exifJPEG(t, 4, 4, binary.BigEndian, 9)},
		{"zero value", exifJPEG(t, 4, 4, binary.LittleEndian, 0)},
		{"wrong type", withAPP1(plain, exifTIFF(binary.BigEndian, 6, 4))},
		{"truncated segment", truncated},
		{"truncated tiff", withAPP1(plain, []byte("Exif\x00\x00II*\x00"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orientation(tt.data); got != 1 {
				t.Errorf("got %d, want 1", got)
			}
		})
	}
}

func TestOrient(t *testing.T) {
	// A 3×2 image with red at the top-left corner and green at the top-right.
	red, green := color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.SetRGBA(0, 0, red)
	src.SetRGBA(2, 0, green)

	tests := []struct {
		o          int
		w, h       int
		red, green image.Point
	}{
		{1, 3, 2, image.Pt(0, 0), image.Pt(2, 0)},
		{2, 3, 2, image.Pt(2, 0), image.Pt(0, 0)},
		{3, 3, 2, image.Pt(2, 1), image.Pt(0, 1)},
		{4, 3, 2, image.Pt(0, 1), image.Pt(2, 1)},
		{5, 2, 3, image.Pt(0, 0), image.Pt(0, 2)},
		{6, 2, 3, image.Pt(1, 0), image.Pt(1, 2)},
		{7, 2, 3, image.Pt(1, 2), image.Pt(1, 0)},
		{8, 2, 3, image.Pt(0, 2), image.Pt(0, 0)},
	}
	for _, tt := range tests {
		got := orient(src, tt.o)
		if b := got.Bounds(); b.Dx() != tt.w || b.Dy() != tt.h {
			t.Errorf("orientation %d: size %dx%d, want %dx%d", tt.o, b.Dx(), b.Dy(), tt.w, tt.h)
			continue
		}
		if got.RGBAAt(tt.red.X, tt.red.Y) != red || got.RGBAAt(tt.green.X, tt.green.Y) != green {
			t.Errorf("orientation %d: corners not at %v and %v", tt.o, tt.red, tt.green)
		}
	}
}
