package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/davidbyttow/govips/v2/vips"
)

func main() {
	vips.Startup(&vips.Config{ConcurrencyLevel: 1})
	defer vips.Shutdown()

	img, err := vips.Black(64, 48)
	if err != nil { panic(err) }
	img.SetString("exif-ifd0-Make", "Apple")
	img.SetString("exif-ifd0-Model", "iPhone 17 Pro")
	img.SetString("exif-ifd2-FNumber", "18/10 (1.8)")
	img.SetString("exif-ifd2-ExposureTime", "1/120 (0.008)")
	img.SetString("exif-ifd3-GPSLatitude", "31/1 13/1 5799/100")
	img.SetString("exif-ifd3-GPSLatitudeRef", "N")
	img.SetString("exif-ifd3-GPSLongitude", "121/1 28/1 2988/100")
	img.SetString("exif-ifd3-GPSLongitudeRef", "E")

	p := vips.NewJpegExportParams()
	p.Quality = 90
	p.StripMetadata = false
	data, _, err := img.ExportJpeg(p)
	if err != nil { panic(err) }
	os.WriteFile("../gps.jpg", data, 0644)
	fmt.Printf("写出 %d 字节\n", len(data))

	// 回读确认
	back, err := vips.NewImageFromBuffer(data)
	if err != nil { panic(err) }
	ex := back.GetExif()
	keys := make([]string, 0, len(ex))
	for k := range ex { keys = append(keys, k) }
	sort.Strings(keys)
	for _, k := range keys { fmt.Printf("  %-34s %s\n", k, ex[k]) }
}
