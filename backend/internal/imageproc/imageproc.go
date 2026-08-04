// Package imageproc wraps libvips (through govips) to do everything that
// happens to an image between "bytes arrived" and "bytes are stored".
//
// Three jobs, in order of importance:
//
//  1. Safety. Every accepted image is decoded and re-encoded from scratch.
//     That destroys polyglot files (a valid GIF that is also a valid HTML
//     page, say) and anything hiding in a trailing chunk, which is the main
//     way an image host turns into a malware host.
//  2. Privacy. EXIF is stripped, which removes GPS coordinates, camera serial
//     numbers, and timestamps that users rarely realise they're publishing.
//     Orientation is applied to the pixels first so stripping doesn't rotate
//     the picture.
//  3. Size. A donated pool is finite, so originals are re-compressed and a
//     WebP plus three thumbnails are generated. AVIF is the best of the lot
//     but roughly an order of magnitude slower to encode, so it is left to
//     the background queue rather than the upload request.
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/gentpan/openimg/backend/internal/models"
)

// Quality settings. These are the usual sweet spots: visually transparent for
// photographs while cutting file size hard.
const (
	jpegQuality  = 82
	webpQuality  = 80
	avifQuality  = 50 // AVIF's scale runs lower than JPEG's for equal quality
	avifEffort   = 4  // 0-9; higher is smaller but much slower
	pngCompress  = 9
	thumbQuality = 78
	// Avatars run a notch above the AVIF default: they are shown small but
	// often against a solid background, where banding in a gradient is far more
	// visible than the same artefact inside a photograph.
	avatarQuality = 62

	// unboundedHeight lets vips_thumbnail size purely by width: libvips fits
	// within both constraints, so a height no real image reaches never binds.
	// It must stay well under VIPS_MAX_COORD (10,000,000), which libvips
	// rejects outright — the tallest image any tier accepts is 30,000px.
	unboundedHeight = 100_000
)

// ThumbWidths are the sizes offered on demand. Nothing is produced at upload;
// the user picks a size and only then does it cost storage.
var ThumbWidths = []struct {
	Name  string
	Width int
}{
	{models.VariantW200, 200},
	{models.VariantW600, 600},
	{models.VariantW1200, 1200},
}

var (
	startOnce sync.Once
	started   bool
)

// Startup initialises libvips. Safe to call more than once; only the first
// call takes effect.
func Startup(concurrency int) error {
	var err error
	startOnce.Do(func() {
		if concurrency < 1 {
			concurrency = 1
		}
		// Logging is routed through the standard logger at warning and above;
		// libvips is chatty at info level.
		vips.LoggingSettings(func(domain string, level vips.LogLevel, msg string) {
			if level <= vips.LogLevelWarning {
				log.Printf("vips[%s]: %s", domain, msg)
			}
		}, vips.LogLevelWarning)
		err = vips.Startup(&vips.Config{
			ConcurrencyLevel: concurrency,
			// The operation cache mostly helps repeated work on the same
			// image, which never happens here — every upload is new bytes.
			// Keeping it small avoids a slow memory climb in a long-running
			// server.
			MaxCacheMem:   32 << 20,
			MaxCacheFiles: 0,
			MaxCacheSize:  50,
		})
		started = err == nil
	})
	return err
}

func Shutdown() {
	if started {
		vips.Shutdown()
	}
}

// Output is one encoded derivative.
type Output struct {
	Ext    string
	MIME   string
	Data   []byte
	Width  int
	Height int
}

func (o Output) Size() int64 { return int64(len(o.Data)) }

// Result is everything produced from one upload.
type Result struct {
	// Primary is the re-encoded original: same visual content, stripped and
	// recompressed. This is what a plain image URL serves.
	Primary Output
	// Variants are keyed by models.Variant* names.
	Variants map[string]Output
	// Animated marks a multi-frame source. Animation is preserved in Primary
	// but thumbnails are stills.
	Animated bool
}

// TotalBytes is what the upload will occupy — the number quota is charged on.
func (r *Result) TotalBytes() int64 {
	n := r.Primary.Size()
	for _, v := range r.Variants {
		n += v.Size()
	}
	return n
}

// VariantBytes is everything full-size that isn't the primary — the webp/avif
// re-encode and any kept original. Derived by subtraction rather than by
// listing names so the three size columns always sum to SizeStored, whatever
// new variant kinds get added later.
func (r *Result) VariantBytes() int64 {
	return r.TotalBytes() - r.Primary.Size() - r.ThumbBytes()
}

func (r *Result) ThumbBytes() int64 {
	var n int64
	for _, t := range ThumbWidths {
		if v, ok := r.Variants[t.Name]; ok {
			n += v.Size()
		}
	}
	return n
}

func (r *Result) VariantNames() string {
	names := make([]string, 0, len(r.Variants))
	// Emit in a stable order so the CSV doesn't churn between uploads.
	for _, v := range []string{models.VariantWebP, models.VariantAVIF} {
		if _, ok := r.Variants[v]; ok {
			names = append(names, v)
		}
	}
	for _, t := range ThumbWidths {
		if _, ok := r.Variants[t.Name]; ok {
			names = append(names, t.Name)
		}
	}
	for name := range r.Variants {
		if models.IsOriginalVariant(name) {
			names = append(names, name)
		}
	}
	return strings.Join(names, ",")
}

// Options constrain a single Process call, sourced from the user's tier.
type Options struct {
	MaxWidth  int
	MaxHeight int
	// SkipVariants processes only the primary image. Used for formats where
	// derivatives make no sense.
	SkipVariants bool
	// Original stores the uploaded bytes verbatim: no re-encode, no metadata
	// stripping, no resize. Thumbnails are still generated because the site
	// renders those, not the original.
	//
	// The caller must still have run Detect() first. Skipping the re-encode
	// means polyglot files survive, so the storage layer's Content-Type and
	// the CDN's nosniff header become the only thing standing between a
	// "picture" and an HTML page.
	Original bool
	// Variant is the single full-size derivative to produce: "", "none",
	// "webp" or "avif".
	Variant string
	// ResizeWidth downscales the stored image to this width when the source is
	// wider. 0 disables it. Never upscales. Ignored in Original mode.
	ResizeWidth int
	// KeepOriginal additionally stores the bytes exactly as uploaded, so the
	// user can still retrieve the untouched file after optimisation. No effect
	// in Original mode, where the primary already is the original.
	KeepOriginal bool
}

// VariantFormat values.
const (
	VariantNone = "none"
)

// ErrTooLarge means the image's pixel dimensions exceed the caller's limits.
// Reported before any full decode, so a decompression bomb costs us a header
// parse rather than gigabytes of RAM.
type ErrTooLarge struct {
	Width, Height, MaxWidth, MaxHeight int
}

func (e *ErrTooLarge) Error() string {
	return fmt.Sprintf("图片尺寸 %dx%d 超出上限 %dx%d", e.Width, e.Height, e.MaxWidth, e.MaxHeight)
}

// ErrUnsupported means libvips has no loader for these bytes, or the format is
// one we refuse to host (SVG in particular: it is a script container, not a
// picture, and cannot be made safe by re-encoding).
type ErrUnsupported struct{ Format string }

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("不支持的图片格式：%s", e.Format)
}

// Detect identifies the format of a buffer without fully decoding it. Returns
// the canonical extension ("jpg", "png", …).
func Detect(buf []byte) (string, error) {
	t := vips.DetermineImageType(buf)
	ext := extForType(t)
	if ext == "" {
		return "", &ErrUnsupported{Format: "unknown"}
	}
	if !allowedType(t) {
		return "", &ErrUnsupported{Format: ext}
	}
	return ext, nil
}

// Process runs the full pipeline on an uploaded buffer.
func Process(buf []byte, opts Options) (*Result, error) {
	imgType := vips.DetermineImageType(buf)
	if !allowedType(imgType) {
		return nil, &ErrUnsupported{Format: pickExt(extForType(imgType), "unknown")}
	}

	if imgType == vips.ImageTypeSVG {
		return nil, &ErrUnsupported{Format: "svg"}
	}

	// Load with animation preserved so Pages() is meaningful. The n=-1 option
	// only exists on multi-page loaders — handing it to jpegload_buffer is a
	// hard error, not a no-op. FailOnError makes libvips reject truncated or
	// corrupt data rather than producing a half-decoded image.
	params := vips.NewImportParams()
	params.FailOnError.Set(true)
	if supportsMultiPage(imgType) {
		params.NumPages.Set(-1)
	}
	// Thumbnails are always stills, so they load without the page option
	// regardless of format.
	stillParams := vips.NewImportParams()
	stillParams.FailOnError.Set(true)

	img, err := vips.LoadImageFromBuffer(buf, params)
	if err != nil {
		return nil, fmt.Errorf("解码失败：%w", err)
	}
	defer img.Close()

	pages := img.Pages()
	animated := pages > 1
	// For a multi-page image, Height() is the height of the whole filmstrip.
	height := img.Height()
	if animated && pages > 0 {
		height /= pages
	}
	width := img.Width()

	if opts.MaxWidth > 0 && opts.MaxHeight > 0 {
		if width > opts.MaxWidth || height > opts.MaxHeight {
			return nil, &ErrTooLarge{
				Width: width, Height: height,
				MaxWidth: opts.MaxWidth, MaxHeight: opts.MaxHeight,
			}
		}
	}

	// Bake orientation into the pixels before metadata is discarded, or every
	// portrait photo from a phone ends up sideways.
	if !animated {
		if err := img.AutoRotate(); err != nil {
			return nil, fmt.Errorf("旋转失败：%w", err)
		}
	}

	// Downscale before encoding. Done after AutoRotate so the width limit
	// applies to the image as the viewer will see it, not to the raw sensor
	// orientation. Animated sources are left alone: resizing a filmstrip
	// in-place would need per-page handling.
	srcWidth, srcHeight := width, height
	if opts.ResizeWidth > 0 && !animated && !opts.Original && width > opts.ResizeWidth {
		scale := float64(opts.ResizeWidth) / float64(width)
		if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
			return nil, fmt.Errorf("缩放失败：%w", err)
		}
		width = img.Width()
		height = img.Height()
	}
	if err := img.RemoveMetadata(); err != nil {
		return nil, fmt.Errorf("移除元数据失败：%w", err)
	}

	if opts.Original {
		// Verbatim passthrough. Dimensions come from the decode we already did.
		res := &Result{
			Primary: Output{
				Ext:    pickExt(extForType(imgType), "bin"),
				MIME:   mimeForType(imgType),
				Data:   buf,
				Width:  width,
				Height: height,
			},
			Variants: map[string]Output{},
			Animated: animated,
		}
		addGridThumb(res, buf, stillParams, width)
		return res, nil
	}

	primary, err := encodePrimary(img, imgType, jpegQuality)
	if err != nil {
		return nil, err
	}
	// Re-encoding can come out *larger* than the input — an already-optimised
	// JPEG, or anything close to noise, has nothing left to squeeze. We can't
	// just keep the original bytes (stripping EXIF and killing polyglots is
	// why we re-encode at all), but we can spend less on it. Step the quality
	// down until we're under the original or run out of steps.
	if isLossy(imgType) && primary.Size() > int64(len(buf)) {
		for _, q := range []int{72, 62} {
			alt, aerr := encodePrimary(img, imgType, q)
			if aerr != nil {
				break
			}
			if alt.Size() < primary.Size() {
				primary = alt
			}
			if primary.Size() <= int64(len(buf)) {
				break
			}
		}
	}
	// PNG has no quality dial, but it does have the filter. Adaptive filtering
	// wins on photographs and can lose on flat synthetic images — screenshots,
	// charts, anything with long runs of identical pixels — where the filter
	// bytes cost more than they save. Only pay for the second encode when the
	// first one came out bigger than the input.
	if imgType == vips.ImageTypePNG && primary.Size() > int64(len(buf)) {
		if alt, aerr := encodePNG(img, vips.PngFilterNone); aerr == nil && alt.Size() < primary.Size() {
			alt.Width, alt.Height = primary.Width, primary.Height
			primary = alt
		}
	}
	primary.Width, primary.Height = width, height

	res := &Result{Primary: primary, Variants: map[string]Output{}, Animated: animated}

	// Keeping the original is deliberately independent of SkipVariants: the
	// point is to hand back the exact bytes the user sent, which matters most
	// for the odd formats that skip derivatives.
	if opts.KeepOriginal && !bytes.Equal(buf, primary.Data) {
		res.Variants[models.OriginalVariant(pickExt(extForType(imgType), "bin"))] = Output{
			Ext:    pickExt(extForType(imgType), "bin"),
			MIME:   mimeForType(imgType),
			Data:   buf,
			Width:  srcWidth,
			Height: srcHeight,
		}
	}

	if opts.SkipVariants {
		return res, nil
	}

	// The grid thumbnail is the one derivative always worth producing up front:
	// without it every gallery view downloads full-size originals. Larger tiers
	// stay on demand.
	addGridThumb(res, buf, stillParams, width)

	// At most one full-size derivative. AVIF is produced by the background
	// queue, so only WebP is handled inline here.
	if opts.Variant == models.VariantWebP && !animated && imgType != vips.ImageTypeWEBP {
		if out, err := encodeWebP(buf, stillParams, 0, webpQuality); err == nil {
			// Only keep it if it actually beats the primary; for small PNGs
			// and already-tight JPEGs it sometimes doesn't, and storing a
			// larger "optimised" copy is pure waste.
			if out.Size() < primary.Size() {
				res.Variants[models.VariantWebP] = out
			}
		} else {
			log.Printf("imageproc: webp variant failed: %v", err)
		}
	}

	return res, nil
}

// AvatarSize is the stored edge length of a profile picture. One size only:
// avatars are shown at 24–36px in the nav and ~64px in settings, so 256 covers
// every use at 2x with room to spare, and a second tier would cost more in
// complexity than it saves in bytes.
const AvatarSize = 256

// MakeAvatar renders a square profile picture.
//
// InterestingAttention picks the crop window by looking for the busiest region
// rather than taking the geometric centre, which is what keeps a face in frame
// when the source is a wide photo. SizeDown means a small source is padded by
// the caller's layout instead of being upscaled into mush.
func MakeAvatar(buf []byte) (*Output, error) {
	img, err := vips.NewThumbnailWithSizeFromBuffer(
		buf, AvatarSize, AvatarSize, vips.InterestingAttention, vips.SizeDown)
	if err != nil {
		return nil, fmt.Errorf("裁剪头像失败：%w", err)
	}
	defer img.Close()

	if err := img.RemoveMetadata(); err != nil {
		return nil, fmt.Errorf("移除元数据失败：%w", err)
	}

	// AVIF. Encoding it is slow enough that the upload pipeline pushes it to a
	// background queue, but that reasoning does not carry here: this is one
	// 256px square, so the encode is milliseconds rather than the seconds a
	// full-size photo costs, and doing it inline keeps the avatar a single
	// object with no "still converting" state to design around.
	p := vips.NewAvifExportParams()
	p.Quality = avatarQuality
	p.Effort = avifEffort
	p.StripMetadata = true

	data, meta, err := img.ExportAvif(p)
	if err != nil {
		return nil, fmt.Errorf("编码头像失败：%w", err)
	}
	return &Output{
		Ext: "avif", MIME: "image/avif", Data: data,
		Width: meta.Width, Height: meta.Height,
	}, nil
}

// GridThumbWidth is the tier generated at upload time.
const GridThumbWidth = 200

// addGridThumb produces the grid thumbnail, skipping sources already at or
// below that width — those render fine as-is.
func addGridThumb(res *Result, buf []byte, params *vips.ImportParams, width int) {
	if width <= GridThumbWidth {
		return
	}
	out, err := encodeWebP(buf, params, GridThumbWidth, thumbQuality)
	if err != nil {
		log.Printf("imageproc: grid thumbnail failed: %v", err)
		return
	}
	res.Variants[models.VariantW200] = out
}

// MakeThumbnail renders one display size on demand. Called from the API when a
// user actually asks for that size, rather than speculatively at upload —
// most images are never viewed at more than one size, and the unused tiers
// were costing 5-23% of stored bytes for nothing.
//
// Returns ErrNotWorthIt when the source is too close to the requested width to
// justify a second copy.
func MakeThumbnail(buf []byte, width int) (*Output, error) {
	imgType := vips.DetermineImageType(buf)
	if !allowedType(imgType) {
		return nil, &ErrUnsupported{Format: pickExt(extForType(imgType), "unknown")}
	}
	params := vips.NewImportParams()
	params.FailOnError.Set(true)

	probe, err := vips.LoadImageFromBuffer(buf, params)
	if err != nil {
		return nil, err
	}
	srcWidth := probe.Width()
	probe.Close()

	if srcWidth <= width {
		return nil, ErrNotWorthIt
	}
	out, err := encodeWebP(buf, params, width, thumbQuality)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ErrNotWorthIt means the requested size is not smaller than the source.
var ErrNotWorthIt = errors.New("请求的尺寸不小于原图，无需生成")

// mimeForType maps a libvips type to the Content-Type the object is stored
// with. In Original mode this is the only thing preventing a polyglot file
// from being served as something a browser will execute.
func mimeForType(t vips.ImageType) string {
	switch t {
	case vips.ImageTypeJPEG:
		return "image/jpeg"
	case vips.ImageTypePNG:
		return "image/png"
	case vips.ImageTypeWEBP:
		return "image/webp"
	case vips.ImageTypeGIF:
		return "image/gif"
	case vips.ImageTypeAVIF:
		return "image/avif"
	case vips.ImageTypeHEIF:
		return "image/heic"
	case vips.ImageTypeTIFF:
		return "image/tiff"
	case vips.ImageTypeBMP:
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// MakeAVIF encodes the AVIF derivative. Split out from Process because it is
// slow enough that it belongs in the background queue.
func MakeAVIF(buf []byte) (*Output, error) {
	params := vips.NewImportParams()
	params.FailOnError.Set(true)

	img, err := vips.LoadImageFromBuffer(buf, params)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	if img.Pages() > 1 {
		return nil, fmt.Errorf("animated source, skipping avif")
	}
	if err := img.AutoRotate(); err != nil {
		return nil, err
	}
	if err := img.RemoveMetadata(); err != nil {
		return nil, err
	}
	p := vips.NewAvifExportParams()
	p.Quality = avifQuality
	p.Effort = avifEffort
	p.StripMetadata = true

	data, meta, err := img.ExportAvif(p)
	if err != nil {
		return nil, err
	}
	return &Output{
		Ext: "avif", MIME: "image/avif", Data: data,
		Width: meta.Width, Height: meta.Height,
	}, nil
}

// encodePrimary re-encodes the original in its own format at the given
// quality. HEIC is the one format we deliberately convert away from —
// browsers can't display it.
// encodePNG exists so the filter can be varied; every other format picks its
// tradeoff with a quality number instead.
//
// govips defaults PngExportParams.Filter to PngFilterNone, which switches off
// PNG's per-row adaptive filtering entirely. Filtering is the whole reason
// deflate gets anywhere on photographic data, so without it the "optimised"
// output is larger than what the user uploaded — measured on a real 2752x1536
// upload: 11,417,407 bytes unfiltered versus 6,326,473 adaptive, against an
// original of 6,348,804. Compression level 9 cannot make up for it; every PNG
// on the site was stored 30-80% larger than it arrived.
func encodePNG(img *vips.ImageRef, filter vips.PngFilter) (Output, error) {
	p := vips.NewPngExportParams()
	p.Compression = pngCompress
	p.StripMetadata = true
	p.Filter = filter
	data, meta, err := img.ExportPng(p)
	return Output{Ext: "png", MIME: "image/png", Data: data, Width: meta.Width, Height: meta.Height}, err
}

func encodePrimary(img *vips.ImageRef, t vips.ImageType, quality int) (Output, error) {
	switch t {
	case vips.ImageTypeJPEG:
		p := vips.NewJpegExportParams()
		p.Quality = quality
		p.StripMetadata = true
		p.OptimizeCoding = true
		p.Interlace = true // progressive: perceived load time on slow links
		data, meta, err := img.ExportJpeg(p)
		return Output{Ext: "jpg", MIME: "image/jpeg", Data: data, Width: meta.Width, Height: meta.Height}, err

	case vips.ImageTypePNG:
		return encodePNG(img, vips.PngFilterAll)

	case vips.ImageTypeWEBP:
		p := vips.NewWebpExportParams()
		p.Quality = scaleForWebP(quality)
		p.StripMetadata = true
		data, meta, err := img.ExportWebp(p)
		return Output{Ext: "webp", MIME: "image/webp", Data: data, Width: meta.Width, Height: meta.Height}, err

	case vips.ImageTypeGIF:
		p := vips.NewGifExportParams()
		p.StripMetadata = true
		data, meta, err := img.ExportGIF(p)
		return Output{Ext: "gif", MIME: "image/gif", Data: data, Width: meta.Width, Height: meta.Height}, err

	case vips.ImageTypeAVIF:
		p := vips.NewAvifExportParams()
		p.Quality = scaleForAVIF(quality)
		p.Effort = avifEffort
		p.StripMetadata = true
		data, meta, err := img.ExportAvif(p)
		return Output{Ext: "avif", MIME: "image/avif", Data: data, Width: meta.Width, Height: meta.Height}, err

	case vips.ImageTypeHEIF:
		// Convert to JPEG: HEIC has no browser support worth speaking of, and
		// a picture nobody can open is not a hosted picture.
		p := vips.NewJpegExportParams()
		p.Quality = quality
		p.StripMetadata = true
		p.OptimizeCoding = true
		data, meta, err := img.ExportJpeg(p)
		return Output{Ext: "jpg", MIME: "image/jpeg", Data: data, Width: meta.Width, Height: meta.Height}, err

	default:
		return Output{}, &ErrUnsupported{Format: pickExt(extForType(t), "unknown")}
	}
}

// encodeWebP renders a WebP at the given width (0 = original size).
//
// Thumbnails go through LoadThumbnailFromBuffer rather than a full decode
// followed by a resize: that path uses shrink-on-load, so a 6000px JPEG is
// never fully expanded in memory just to produce a 200px thumbnail. It also
// applies orientation itself. unboundedHeight with SizeDown means "fit the
// width, keep the aspect ratio, never upscale".
func encodeWebP(buf []byte, params *vips.ImportParams, width, quality int) (Output, error) {
	var img *vips.ImageRef
	var err error
	if width > 0 {
		img, err = vips.LoadThumbnailFromBuffer(buf, width, unboundedHeight,
			vips.InterestingNone, vips.SizeDown, params)
	} else {
		img, err = vips.LoadImageFromBuffer(buf, params)
		if err == nil {
			err = img.AutoRotate()
		}
	}
	if err != nil {
		if img != nil {
			img.Close()
		}
		return Output{}, err
	}
	defer img.Close()

	if err := img.RemoveMetadata(); err != nil {
		return Output{}, err
	}
	p := vips.NewWebpExportParams()
	p.Quality = quality
	p.StripMetadata = true
	p.ReductionEffort = 4

	data, meta, err := img.ExportWebp(p)
	if err != nil {
		return Output{}, err
	}
	return Output{
		Ext: "webp", MIME: "image/webp", Data: data,
		Width: meta.Width, Height: meta.Height,
	}, nil
}

// allowedType is the whitelist. SVG is excluded on purpose: it is XML that can
// carry script, and unlike raster formats it cannot be neutralised by
// re-encoding. PDF and the other document types libvips can open have no
// business in an image host either.
func allowedType(t vips.ImageType) bool {
	switch t {
	case vips.ImageTypeJPEG, vips.ImageTypePNG, vips.ImageTypeWEBP,
		vips.ImageTypeGIF, vips.ImageTypeAVIF, vips.ImageTypeHEIF,
		vips.ImageTypeTIFF, vips.ImageTypeBMP:
		return true
	default:
		return false
	}
}

// isLossy reports whether stepping the quality down actually changes the
// output size. PNG and GIF are lossless — retrying them at "lower quality"
// would just burn CPU for an identical result.
func isLossy(t vips.ImageType) bool {
	switch t {
	case vips.ImageTypeJPEG, vips.ImageTypeWEBP, vips.ImageTypeAVIF, vips.ImageTypeHEIF:
		return true
	default:
		return false
	}
}

// scaleForWebP / scaleForAVIF map a JPEG-style quality onto each codec's own
// scale, which sits lower for the same perceived quality.
func scaleForWebP(jpegQ int) int { return clampQ(jpegQ - 2) }
func scaleForAVIF(jpegQ int) int { return clampQ(jpegQ - 32) }

func clampQ(q int) int {
	if q < 30 {
		return 30
	}
	if q > 100 {
		return 100
	}
	return q
}

// supportsMultiPage reports whether a loader accepts the `n` (page count)
// option. Passing it to a single-page loader like JPEG fails the whole load.
func supportsMultiPage(t vips.ImageType) bool {
	switch t {
	case vips.ImageTypeGIF, vips.ImageTypeWEBP, vips.ImageTypeHEIF,
		vips.ImageTypeAVIF, vips.ImageTypeTIFF, vips.ImageTypePDF:
		return true
	default:
		return false
	}
}

func extForType(t vips.ImageType) string {
	switch t {
	case vips.ImageTypeJPEG:
		// "jpg", not "jpeg": the encoder hard-codes "jpg" for what it writes,
		// and a stored original ending .orig.jpeg beside a primary ending .jpg
		// is the kind of inconsistency that eventually becomes a bug report.
		return "jpg"
	case vips.ImageTypePNG:
		return "png"
	case vips.ImageTypeWEBP:
		return "webp"
	case vips.ImageTypeGIF:
		return "gif"
	case vips.ImageTypeAVIF:
		return "avif"
	case vips.ImageTypeHEIF:
		return "heic"
	case vips.ImageTypeTIFF:
		return "tiff"
	case vips.ImageTypeBMP:
		return "bmp"
	case vips.ImageTypeSVG:
		return "svg"
	case vips.ImageTypePDF:
		return "pdf"
	default:
		return ""
	}
}

func pickExt(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
