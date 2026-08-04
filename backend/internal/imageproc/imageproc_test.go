package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/gentpan/openimg/backend/internal/models"
)

func TestMain(m *testing.M) {
	if err := Startup(1); err != nil {
		panic(err)
	}
	defer Shutdown()
	m.Run()
}

// synthJPEG builds a gradient large enough to exercise the thumbnail ladder.
func synthJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode source: %v", err)
	}
	return buf.Bytes()
}

func synthPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: 40, B: 90, A: uint8(x % 256)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source: %v", err)
	}
	return buf.Bytes()
}

func TestDetect(t *testing.T) {
	if ext, err := Detect(synthJPEG(t, 64, 64)); err != nil || ext != "jpg" {
		t.Fatalf("Detect(jpeg) = %q, %v; want jpeg, nil", ext, err)
	}
	if ext, err := Detect(synthPNG(t, 64, 64)); err != nil || ext != "png" {
		t.Fatalf("Detect(png) = %q, %v; want png, nil", ext, err)
	}
}

// SVG is the format the whole re-encode strategy can't neutralise, so it must
// be refused outright rather than processed.
func TestDetectRejectsSVG(t *testing.T) {
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` +
		`<script>alert(1)</script><rect width="10" height="10"/></svg>`)
	if _, err := Detect(svg); err == nil {
		t.Fatal("Detect(svg) succeeded; SVG must be rejected")
	}
}

func TestDetectRejectsNonImage(t *testing.T) {
	if _, err := Detect([]byte("#!/bin/sh\necho hello\n")); err == nil {
		t.Fatal("Detect(shell script) succeeded; want error")
	}
}

func TestProcessGeneratesVariants(t *testing.T) {
	// 2400px wide so every tier clears the 2x ratio rule.
	src := synthJPEG(t, 2400, 1350)
	res, err := Process(src, Options{MaxWidth: 8000, MaxHeight: 8000})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Primary.Ext != "jpg" {
		t.Errorf("primary ext = %q, want jpg", res.Primary.Ext)
	}
	if res.Primary.Width != 2400 || res.Primary.Height != 1350 {
		t.Errorf("primary dims = %dx%d, want 2400x1350", res.Primary.Width, res.Primary.Height)
	}
	if len(res.Primary.Data) == 0 {
		t.Fatal("primary has no data")
	}
	// Only the grid thumbnail is produced up front. The larger tiers are the
	// caller's job now, via MakeThumbnail.
	v, ok := res.Variants[models.VariantW200]
	if !ok {
		t.Fatal("missing grid thumbnail w200")
	}
	if v.Ext != "webp" {
		t.Errorf("w200 ext = %q, want webp", v.Ext)
	}
	if v.Width != 200 {
		t.Errorf("w200 width = %d, want 200", v.Width)
	}
	for _, unwanted := range []string{models.VariantW600, models.VariantW1200} {
		if _, ok := res.Variants[unwanted]; ok {
			t.Errorf("%s generated at upload; it should be on demand only", unwanted)
		}
	}
	if res.TotalBytes() <= 0 {
		t.Error("TotalBytes should be positive")
	}
}

// A larger tier requested on demand still has to earn its bytes: asking for a
// 1200px copy of a 1200px original should be refused, not silently duplicated.
func TestMakeThumbnailRefusesNonShrink(t *testing.T) {
	src := synthJPEG(t, 1000, 600)
	if _, err := MakeThumbnail(src, 1200); !errors.Is(err, ErrNotWorthIt) {
		t.Errorf("MakeThumbnail(1200) on a 1000px source = %v, want ErrNotWorthIt", err)
	}
	out, err := MakeThumbnail(src, 600)
	if err != nil {
		t.Fatalf("MakeThumbnail(600): %v", err)
	}
	if out.Width != 600 || out.Ext != "webp" {
		t.Errorf("on-demand tier = %dpx/%s, want 600px/webp", out.Width, out.Ext)
	}
}

// A small source must not be upscaled into thumbnails that are bigger than the
// original — that would spend space to make the picture worse.
func TestProcessSkipsUpscaling(t *testing.T) {
	res, err := Process(synthJPEG(t, 150, 150), Options{MaxWidth: 8000, MaxHeight: 8000})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	for _, name := range []string{models.VariantW200, models.VariantW600, models.VariantW1200} {
		if _, ok := res.Variants[name]; ok {
			t.Errorf("variant %s generated for a 150px source", name)
		}
	}
}

func TestProcessRejectsOversized(t *testing.T) {
	src := synthJPEG(t, 900, 900)
	_, err := Process(src, Options{MaxWidth: 800, MaxHeight: 800})
	if err == nil {
		t.Fatal("Process accepted an oversized image")
	}
	var tooLarge *ErrTooLarge
	if !errors.As(err, &tooLarge) {
		t.Fatalf("error = %T (%v), want *ErrTooLarge", err, err)
	}
}

// EXIF must not survive: it routinely carries GPS coordinates that users have
// no idea they're publishing.
func TestProcessStripsMetadata(t *testing.T) {
	src := synthJPEG(t, 400, 300)
	// Splice a minimal APP1/Exif segment in after SOI.
	exif := []byte{0xFF, 0xE1, 0x00, 0x10, 'E', 'x', 'i', 'f', 0, 0, 'M', 'M', 0, 42, 0, 0, 0, 8}
	withExif := append(append(append([]byte{}, src[:2]...), exif...), src[2:]...)

	res, err := Process(withExif, Options{MaxWidth: 8000, MaxHeight: 8000})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if bytes.Contains(res.Primary.Data, []byte("Exif")) {
		t.Error("primary output still contains an Exif marker")
	}
}
