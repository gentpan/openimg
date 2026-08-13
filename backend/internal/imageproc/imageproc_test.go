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
	v, ok := res.Variants[models.VariantW600]
	if !ok {
		t.Fatal("missing grid thumbnail w600")
	}
	if v.Ext != "webp" {
		t.Errorf("w600 ext = %q, want webp", v.Ext)
	}
	if v.Width != GridThumbWidth {
		t.Errorf("w600 width = %d, want %d", v.Width, GridThumbWidth)
	}
	for _, unwanted := range []string{models.VariantW200, models.VariantW1200} {
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

// synthPhotoPNG builds rows that differ from one another but vary smoothly
// across each row — the shape of photographic data, and the only case where
// PNG's row filters earn their keep. A flat or exactly-repeating pattern
// compresses fine unfiltered, so it would not catch a filter regression.
func synthPhotoPNG(t *testing.T, w, h int) []byte { return synthPhotoPNGJitter(t, w, h, 4) }

func synthPhotoPNGJitter(t *testing.T, w, h, amp int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(0x9E3779B9)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			jitter := 0
			if amp > 0 {
				jitter = int(seed>>26)%(2*amp) - amp // small, so neighbours stay close
			}
			clamp := func(v int) uint8 {
				if v < 0 {
					return 0
				}
				if v > 255 {
					return 255
				}
				return uint8(v)
			}
			img.Set(x, y, color.RGBA{
				R: clamp(x*200/w + y*40/h + jitter),
				G: clamp(y*200/h + jitter),
				B: clamp((x+y)*160/(w+h) + 40 + jitter),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source: %v", err)
	}
	return buf.Bytes()
}

// A stored PNG must never be meaningfully larger than the one that was
// uploaded. It was: govips defaults the export filter to PngFilterNone, and
// every photographic PNG on the site landed 30-80% bigger than it arrived,
// with the user's quota charged for the difference. Nothing in the suite
// asserted on output size, so the whole thing was invisible.
func TestProcessPNGDoesNotInflate(t *testing.T) {
	src := synthPhotoPNG(t, 900, 600)
	res, err := Process(src, Options{ResizeWidth: 4000})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	in, out := int64(len(src)), res.Primary.Size()
	ratio := float64(out) / float64(in)
	t.Logf("输入 %d 字节 → 主图 %d 字节（%.1f%%）", in, out, ratio*100-100)
	if ratio > 1.05 {
		t.Fatalf("主图比原图大 %.1f%%（%d → %d 字节）；PNG 导出的 Filter 可能又退回 PngFilterNone 了",
			ratio*100-100, in, out)
	}
}

// The grid thumbnail used to key off width alone, on the reasoning that
// anything ≤600px "renders fine as-is". That holds for width and not for
// bytes: a narrow, tall page is under the cut while still being megabytes,
// and the gallery would download the whole thing to paint a 230px card.
func TestGridThumbTriggersOnSizeNotJustWidth(t *testing.T) {
	// 500px wide — under the width trigger — but tall enough to clear 1 MB.
	src := synthPhotoPNGJitter(t, 500, 3200, 4)
	if int64(len(src)) < GridThumbMinBytes {
		t.Fatalf("夹具只有 %d 字节，没到 %d 的触发线，测不到东西", len(src), GridThumbMinBytes)
	}
	res, err := Process(src, Options{})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	v, ok := res.Variants[models.VariantW600]
	if !ok {
		t.Fatalf("宽 500 但 %d 字节的图没有生成缩略图", len(src))
	}
	if v.Width > 500 {
		t.Errorf("缩略图被放大到 %dpx；SizeDown 应该只缩不放", v.Width)
	}
	t.Logf("源 %d 字节 → 主图 %d → 缩略图 %d 字节 (%dx%d)",
		len(src), res.Primary.Size(), v.Size(), v.Width, v.Height)
}

// Small and already efficient: generating anything here would add an object
// that costs quota without ever being the cheaper thing to load.
func TestGridThumbSkippedWhenSmallAndLight(t *testing.T) {
	src := synthPhotoPNGJitter(t, 400, 300, 4)
	res, err := Process(src, Options{})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if v, ok := res.Variants[models.VariantW600]; ok {
		t.Errorf("小图不该生成缩略图，却产出了 %d 字节（主图 %d）", v.Size(), res.Primary.Size())
	}
}

// Whatever triggers generation, the thumbnail only earns its place by being
// smaller than the image it replaces.
func TestGridThumbNeverLargerThanPrimary(t *testing.T) {
	for _, d := range []struct{ w, h int }{{500, 3200}, {1600, 900}, {2400, 1350}} {
		src := synthPhotoPNGJitter(t, d.w, d.h, 4)
		res, err := Process(src, Options{})
		if err != nil {
			t.Fatalf("%dx%d Process: %v", d.w, d.h, err)
		}
		if v, ok := res.Variants[models.VariantW600]; ok && v.Size() >= res.Primary.Size() {
			t.Errorf("%dx%d 的缩略图 %d 字节 ≥ 主图 %d 字节，不该保留",
				d.w, d.h, v.Size(), res.Primary.Size())
		}
	}
}

// 转换语义:选了 WebP 后主图就是 WebP,不保留原格式主图,也没有多余的
// 附加变体(缩略图除外)。
func TestProcessConvertsPrimaryToWebP(t *testing.T) {
	src := synthPNG(t, 1200, 800)
	res, err := Process(src, Options{MaxWidth: 8000, MaxHeight: 8000, Variant: models.VariantWebP})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Primary.Ext != "webp" {
		t.Fatalf("primary ext = %q, want webp(转换语义:主图直接变目标格式)", res.Primary.Ext)
	}
	if res.Primary.MIME != "image/webp" {
		t.Errorf("primary mime = %q, want image/webp", res.Primary.MIME)
	}
	// RIFF....WEBP 魔数,证明字节真是 WebP 而不只是改了扩展名
	d := res.Primary.Data
	if len(d) < 12 || string(d[0:4]) != "RIFF" || string(d[8:12]) != "WEBP" {
		t.Error("primary bytes are not WebP")
	}
	if _, ok := res.Variants[models.VariantWebP]; ok {
		t.Error("webp variant present alongside webp primary; conversion should replace, not append")
	}
	if res.Primary.Width != 1200 || res.Primary.Height != 800 {
		t.Errorf("primary dims = %dx%d, want 1200x800", res.Primary.Width, res.Primary.Height)
	}
}

func TestProcessConvertsPrimaryToAVIF(t *testing.T) {
	src := synthJPEG(t, 800, 600)
	res, err := Process(src, Options{MaxWidth: 8000, MaxHeight: 8000, Variant: models.VariantAVIF})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Primary.Ext != "avif" {
		t.Fatalf("primary ext = %q, want avif", res.Primary.Ext)
	}
	if _, ok := res.Variants[models.VariantAVIF]; ok {
		t.Error("avif variant present alongside avif primary")
	}
}

// 原图模式下转换设置不参与:字节原样保存。
func TestProcessOriginalModeIgnoresConversion(t *testing.T) {
	src := synthPNG(t, 400, 300)
	res, err := Process(src, Options{MaxWidth: 8000, MaxHeight: 8000, Original: true, Variant: models.VariantWebP})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Primary.Ext != "png" {
		t.Errorf("primary ext = %q, want png(原图模式不转换)", res.Primary.Ext)
	}
}
