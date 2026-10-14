package media

import (
	"bytes"
	"encoding/binary"
	"image"
)

// orientation returns the EXIF Orientation (1–8) of a JPEG, or 1 when it has none or
// the metadata cannot be read. It walks the JPEG segments up to the image data, finds
// the APP1 "Exif" segment and reads tag 0x0112 from the TIFF header's first IFD.
func orientation(jpeg []byte) int {
	if len(jpeg) < 4 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(jpeg); {
		if jpeg[i] != 0xFF {
			return 1
		}
		marker := jpeg[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan or end of image: no EXIF
			return 1
		}
		end := i + 2 + int(binary.BigEndian.Uint16(jpeg[i+2:]))
		if end > len(jpeg) || end < i+4 {
			return 1
		}
		if seg := jpeg[i+4 : end]; marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i = end
	}
	return 1
}

func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:]) != 42 {
		return 1
	}
	ifd := int64(order.Uint32(tiff[4:]))
	if ifd > int64(len(tiff)-2) {
		return 1
	}
	n := int(order.Uint16(tiff[ifd:]))
	for e := int(ifd) + 2; n > 0 && e+12 <= len(tiff); e, n = e+12, n-1 {
		const tagOrientation, typeShort = 0x0112, 3
		if order.Uint16(tiff[e:]) != tagOrientation {
			continue
		}
		if order.Uint16(tiff[e+2:]) != typeShort {
			return 1
		}
		if v := int(order.Uint16(tiff[e+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}

// orient applies an EXIF orientation, returning an image that displays upright.
// Orientations 5–8 swap width and height.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	// at maps a destination pixel to the source pixel that belongs there.
	at := map[int]func(x, y int) (int, int){
		2: func(x, y int) (int, int) { return w - 1 - x, y },         // mirror horizontally
		3: func(x, y int) (int, int) { return w - 1 - x, h - 1 - y }, // rotate 180°
		4: func(x, y int) (int, int) { return x, h - 1 - y },         // mirror vertically
		5: func(x, y int) (int, int) { return y, x },                 // transpose
		6: func(x, y int) (int, int) { return y, h - 1 - x },         // rotate 90° clockwise
		7: func(x, y int) (int, int) { return w - 1 - y, h - 1 - x }, // transverse
		8: func(x, y int) (int, int) { return w - 1 - y, x },         // rotate 90° counter-clockwise
	}[o]
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	b := src.Bounds().Min
	for y := range dh {
		for x := range dw {
			sx, sy := at(x, y)
			dst.SetRGBA(x, y, src.RGBAAt(b.X+sx, b.Y+sy))
		}
	}
	return dst
}
