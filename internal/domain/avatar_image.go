package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// The avatar pipeline (ADR-0007, docs/design/iam-scope.md "头像"): a user's file or a directory
// photo goes in, a 256px square JPEG without any metadata comes out, or an error code says
// why the file was refused. Nothing that was in the original file survives except pixels.

const (
	// MaxAvatarBytes is the largest file accepted (the HTTP layer reads at most one byte more).
	MaxAvatarBytes = 2 << 20
	// maxAvatarPixels is checked on the header before the picture is decoded, so a small file that
	// announces a huge picture (a decompression bomb) never gets the memory.
	maxAvatarPixels = 25_000_000
	avatarSize      = 256
	avatarQuality   = 85
	avatarHashLen   = 16 // hex digits of the content hash that go into the URL
)

type avatarFormat int

const (
	formatJPEG avatarFormat = iota + 1
	formatPNG
	formatWebP
)

// sniffAvatar identifies the format from the first bytes of the file. The name and the
// Content-Type the client sent are never looked at.
func sniffAvatar(b []byte) (avatarFormat, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return formatJPEG, true
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return formatPNG, true
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return formatWebP, true
	}
	return 0, false
}

func avatarTooLarge() *Error {
	return Validation(Invalid("avatar", CodeAvatarTooLarge, map[string]any{"maxBytes": MaxAvatarBytes, "maxPixels": maxAvatarPixels}))
}

func avatarUnsupported() *Error {
	return Validation(Invalid("avatar", CodeAvatarUnsupportedType, map[string]any{"allowed": []string{"jpeg", "png", "webp"}}))
}

// processAvatar turns an uploaded file into the stored avatar: a 256x256 JPEG (the centre
// square of the picture, turned upright by its EXIF orientation, transparency flattened onto
// white, light theme only) and the content hash that goes into the avatar URL. The errors are
// validation errors on the field "avatar".
func processAvatar(data []byte) (jpg []byte, hash string, err error) {
	if len(data) > MaxAvatarBytes {
		return nil, "", avatarTooLarge()
	}
	format, ok := sniffAvatar(data)
	if !ok {
		return nil, "", avatarUnsupported()
	}

	// Header first: the dimensions are known before a single pixel is decoded.
	var cfg image.Config
	switch format {
	case formatJPEG:
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case formatPNG:
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case formatWebP:
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", avatarUnsupported()
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxAvatarPixels {
		return nil, "", avatarTooLarge()
	}

	var src image.Image
	switch format {
	case formatJPEG:
		src, err = jpeg.Decode(bytes.NewReader(data))
	case formatPNG:
		src, err = png.Decode(bytes.NewReader(data))
	case formatWebP:
		src, err = webp.Decode(bytes.NewReader(data))
	}
	if err != nil { // the file starts like a picture but is not one
		return nil, "", avatarUnsupported()
	}

	// The centre square of the stored picture is also the centre square of the upright one, so
	// crop and shrink first (cheap) and turn the 256px result afterwards.
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)
	small := image.NewNRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	xdraw.CatmullRom.Scale(small, small.Bounds(), src, crop, xdraw.Src, nil)

	canvas := image.NewRGBA(small.Bounds())
	xdraw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.Draw(canvas, canvas.Bounds(), small, image.Point{}, xdraw.Over)

	upright := orient(canvas, exifOrientation(data, format))

	var out bytes.Buffer // the encoder writes no EXIF, comments or any other metadata
	if err := jpeg.Encode(&out, upright, &jpeg.Options{Quality: avatarQuality}); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(out.Bytes())
	return out.Bytes(), hex.EncodeToString(sum[:])[:avatarHashLen], nil
}

// orient turns a square picture upright. o is the EXIF orientation 1-8 (anything else = 1),
// the transformation that must be applied to the stored picture to display it correctly:
// 2 mirror, 3 rotate 180, 4 flip vertically, 5 transpose, 6 rotate 90 clockwise, 7 transverse,
// 8 rotate 90 counter-clockwise.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	n := src.Bounds().Dx() // square
	dst := image.NewRGBA(src.Bounds())
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var dx, dy int // where the pixel (x, y) of the stored picture is displayed
			switch o {
			case 2:
				dx, dy = n-1-x, y
			case 3:
				dx, dy = n-1-x, n-1-y
			case 4:
				dx, dy = x, n-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = n-1-y, x
			case 7:
				dx, dy = n-1-y, n-1-x
			case 8:
				dx, dy = y, n-1-x
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(x, y))
		}
	}
	return dst
}

// exifOrientation reads the orientation tag from the EXIF block of the file (0 when there is
// none or it cannot be read). JPEG keeps it in an APP1 segment, PNG in an eXIf chunk, WebP in
// an EXIF chunk.
func exifOrientation(data []byte, format avatarFormat) int {
	switch format {
	case formatJPEG:
		return tiffOrientation(jpegExif(data))
	case formatPNG:
		return tiffOrientation(pngExif(data))
	case formatWebP:
		return tiffOrientation(webpExif(data))
	}
	return 0
}

func jpegExif(b []byte) []byte {
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return nil
		}
		marker := b[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan or end: no more headers
			return nil
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8) { // markers without a length
			i += 2
			continue
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			return nil
		}
		if payload := b[i+4 : i+2+n]; marker == 0xE1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			return payload[6:]
		}
		i += 2 + n
	}
	return nil
}

func pngExif(b []byte) []byte {
	for i := 8; i+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[i:]))
		kind := string(b[i+4 : i+8])
		if n < 0 || i+8+n > len(b) {
			return nil
		}
		if kind == "eXIf" {
			return b[i+8 : i+8+n]
		}
		if kind == "IDAT" {
			return nil
		}
		i += 12 + n // length, type, data, CRC
	}
	return nil
}

func webpExif(b []byte) []byte {
	for i := 12; i+8 <= len(b); {
		n := int(binary.LittleEndian.Uint32(b[i+4:]))
		if n < 0 || i+8+n > len(b) {
			return nil
		}
		if string(b[i:i+4]) == "EXIF" {
			return bytes.TrimPrefix(b[i+8:i+8+n], []byte("Exif\x00\x00"))
		}
		i += 8 + n + n%2 // chunks are padded to an even size
	}
	return nil
}

// tiffOrientation finds the Orientation tag (0x0112) in the first IFD of a TIFF block.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(t[2:]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 0
	}
	count := int(order.Uint16(t[ifd:]))
	for e := 0; e < count; e++ {
		at := ifd + 2 + e*12
		if at+12 > len(t) {
			return 0
		}
		if order.Uint16(t[at:]) == 0x0112 && order.Uint16(t[at+2:]) == 3 { // SHORT
			if v := int(order.Uint16(t[at+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 0
		}
	}
	return 0
}
