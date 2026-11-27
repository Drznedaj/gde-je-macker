// Package photos turns an uploaded image into a stripped, resized WebP.
//
// Re-encoding from decoded pixels is what removes EXIF (GPS, device, time):
// no metadata from the input is ever copied to the output. The EXIF
// orientation is read first so rotated phone photos stay upright.
package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"
)

const (
	// MaxUploadBytes is the largest accepted upload.
	MaxUploadBytes = 10 << 20
	// MaxSide is the longest edge of the stored image.
	MaxSide = 1280
	// maxPixels guards against decompression bombs.
	maxPixels = 40_000_000
)

var (
	ErrUnsupported = errors.New("unsupported image type")
	ErrTooLarge    = errors.New("image too large")
	ErrCorrupt     = errors.New("corrupt image")
)

// Result is a re-encoded photo.
type Result struct {
	WebP          []byte
	Width, Height int
}

// Process reads an upload (JPEG or PNG) and returns a resized, metadata-free WebP.
func Process(r io.Reader) (Result, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > MaxUploadBytes {
		return Result{}, ErrTooLarge
	}
	isJPEG := false
	switch http.DetectContentType(data) {
	case "image/jpeg":
		isJPEG = true
	case "image/png":
	default:
		return Result{}, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrCorrupt
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return Result{}, ErrTooLarge
	}
	var src image.Image
	if isJPEG {
		src, err = jpeg.Decode(bytes.NewReader(data))
	} else {
		src, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return Result{}, ErrCorrupt
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if m := max(w, h); m > MaxSide {
		w, h = max(1, w*MaxSide/m), max(1, h*MaxSide/m)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var out image.Image = dst
	if isJPEG {
		out = orient(dst, jpegOrientation(data))
	}
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, out, nil); err != nil {
		return Result{}, err
	}
	ob := out.Bounds()
	return Result{WebP: buf.Bytes(), Width: ob.Dx(), Height: ob.Dy()}, nil
}

// jpegOrientation returns the EXIF orientation (1-8), or 1 if absent.
func jpegOrientation(d []byte) int {
	i := 2
	for i+4 <= len(d) && d[i] == 0xFF {
		marker := d[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		n := int(binary.BigEndian.Uint16(d[i+2:]))
		if n < 2 || i+2+n > len(d) {
			return 1
		}
		seg := d[i+4 : i+2+n]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

func tiffOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[off:]))
	for k := 0; k < count; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient applies an EXIF orientation so the pixels are upright.
func orient(src *image.NRGBA, o int) image.Image {
	if o <= 1 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.SetNRGBA(nx, ny, src.NRGBAAt(x, y))
		}
	}
	return dst
}
