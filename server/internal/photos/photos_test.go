package photos

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

const gpsSentinel = "GPS 44.8125N 20.4612E SENTINEL-CAMERA-9917"

// twoToneImage is red on the left half and blue on the right half.
func twoToneImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	return img
}

// jpegWithEXIF returns a JPEG whose APP1 segment carries an orientation and
// a text tag standing in for GPS/device metadata.
func jpegWithEXIF(t *testing.T, img image.Image, orientation uint16) []byte {
	t.Helper()
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, img, nil); err != nil {
		t.Fatal(err)
	}
	bo := binary.BigEndian
	var tiff bytes.Buffer
	tiff.WriteString("MM")
	_ = binary.Write(&tiff, bo, uint16(42))
	_ = binary.Write(&tiff, bo, uint32(8))
	_ = binary.Write(&tiff, bo, uint16(2)) // two IFD entries, sorted by tag
	str := gpsSentinel + "\x00"
	// 0x010F Make (ASCII), data stored after the IFD at offset 38
	w := func(vals ...any) {
		for _, v := range vals {
			_ = binary.Write(&tiff, bo, v)
		}
	}
	w(uint16(0x010F), uint16(2), uint32(len(str)), uint32(38))
	// 0x0112 Orientation (SHORT)
	w(uint16(0x0112), uint16(3), uint32(1), orientation, uint16(0))
	_ = binary.Write(&tiff, bo, uint32(0))
	tiff.WriteString(str)

	seg := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	app1 = append(app1, seg...)

	out := append([]byte{}, plain.Bytes()[:2]...) // SOI
	out = append(out, app1...)
	return append(out, plain.Bytes()[2:]...)
}

func TestEXIFRemovedAndWebP(t *testing.T) {
	in := jpegWithEXIF(t, twoToneImage(40, 20), 1)
	if !bytes.Contains(in, []byte(gpsSentinel)) {
		t.Fatal("test input should carry the EXIF sentinel")
	}
	res, err := Process(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(res.WebP, []byte("RIFF")) || string(res.WebP[8:12]) != "WEBP" {
		t.Fatal("output is not WebP")
	}
	for _, bad := range []string{gpsSentinel, "SENTINEL", "Exif", "EXIF", "GPS"} {
		if bytes.Contains(res.WebP, []byte(bad)) {
			t.Fatalf("output still contains %q", bad)
		}
	}
	if res.Width != 40 || res.Height != 20 {
		t.Fatalf("size %dx%d", res.Width, res.Height)
	}
}

func TestResizeLimitsLongestSide(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2560, 1280))); err != nil {
		t.Fatal(err)
	}
	res, err := Process(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != MaxSide || res.Height != MaxSide/2 {
		t.Fatalf("size %dx%d", res.Width, res.Height)
	}
}

func TestOrientationApplied(t *testing.T) {
	// Orientation 6: rotate 90 degrees clockwise, so the red left half ends up on top.
	res, err := Process(bytes.NewReader(jpegWithEXIF(t, twoToneImage(40, 20), 6)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 20 || res.Height != 40 {
		t.Fatalf("size %dx%d", res.Width, res.Height)
	}
	img, err := nativewebp.Decode(bytes.NewReader(res.WebP))
	if err != nil {
		t.Fatal(err)
	}
	topR, _, _, _ := img.At(10, 2).RGBA()
	_, _, botB, _ := img.At(10, 37).RGBA()
	if topR>>8 < 200 || botB>>8 < 200 {
		t.Fatalf("unexpected orientation: top R=%d bottom B=%d", topR>>8, botB>>8)
	}
}

func TestRejects(t *testing.T) {
	if _, err := Process(strings.NewReader("<html>not an image</html>")); err != ErrUnsupported {
		t.Fatalf("got %v", err)
	}
	if _, err := Process(bytes.NewReader([]byte("\xff\xd8\xff\xe0garbage"))); err != ErrCorrupt {
		t.Fatalf("got %v", err)
	}
	if _, err := Process(bytes.NewReader(make([]byte, MaxUploadBytes+1))); err != ErrTooLarge {
		t.Fatalf("got %v", err)
	}
}
