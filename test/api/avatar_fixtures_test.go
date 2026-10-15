package api

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// Image fixtures for the avatar tests. Everything is generated here or read from testdata, so
// the tests can say exactly what is in the file they upload.

var (
	colRed  = color.RGBA{R: 220, A: 255}
	colBlue = color.RGBA{B: 220, A: 255}
)

// twoBand is w x h with the top half red and the bottom half blue.
func twoBand(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		c := colRed
		if y >= h/2 {
			c = colBlue
		}
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func jpegBytes(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngBytes(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// solid is a w x h image of one colour (alpha included: use a zero-alpha colour for transparency).
func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// jpegSegment is a marker segment (marker byte after 0xFF) with the given payload.
func jpegSegment(marker byte, payload []byte) []byte {
	seg := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// withJPEGMetadata puts an EXIF segment (orientation 1-8) and a comment that looks like a
// location right after the SOI of a JPEG.
func withJPEGMetadata(jpg []byte, orientation uint16) []byte {
	exif := []byte("Exif\x00\x00")
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8} // big endian, IFD0 at offset 8
	tiff = append(tiff, 0, 1)                   // one entry
	tiff = append(tiff, 0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(orientation>>8), byte(orientation), 0, 0)
	tiff = append(tiff, 0, 0, 0, 0) // no next IFD
	out := append([]byte{}, jpg[:2]...)
	out = append(out, jpegSegment(0xE1, append(exif, tiff...))...)
	out = append(out, jpegSegment(0xFE, []byte("SECRET-COMMENT-GPS-31.23N"))...)
	return append(out, jpg[2:]...)
}

// jpegClaimingSize rewrites the frame header of a JPEG so it claims to be w x h pixels. The
// pixel data is not touched: a decompression bomb is a small file that announces a huge image.
func jpegClaimingSize(t testing.TB, jpg []byte, w, h uint16) []byte {
	t.Helper()
	out := bytes.Clone(jpg)
	i := bytes.Index(out, []byte{0xFF, 0xC0}) // baseline frame header: FF C0 len(2) precision(1) height(2) width(2)
	if i < 0 {
		t.Fatal("no SOF0 marker in the fixture JPEG")
	}
	binary.BigEndian.PutUint16(out[i+5:], h)
	binary.BigEndian.PutUint16(out[i+7:], w)
	return out
}

// webpFixture is a small lossy WebP (golang.org/x/image testdata, BSD licence).
func webpFixture(t testing.TB) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/avatar.webp")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// nearColor reports whether the pixel at (x, y) is close to want (the avatar is re-encoded lossily).
func nearColor(img image.Image, x, y int, want color.RGBA) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	d := func(got uint32, want uint8) int {
		v := int(got>>8) - int(want)
		if v < 0 {
			v = -v
		}
		return v
	}
	return d(r, want.R) < 40 && d(g, want.G) < 40 && d(b, want.B) < 40
}
