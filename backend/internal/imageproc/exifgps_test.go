package imageproc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

// gpsJPEG builds a JPEG carrying both location and the camera settings a
// photographer would be annoyed to lose.
//
// The two variants are deliberately different sizes. vips.Black is a cached
// operation, so calling it twice with identical arguments hands back the same
// image — including the EXIF the previous caller wrote onto it. With matching
// dimensions the "no GPS" fixture silently arrives carrying the coordinates
// from the test before it, and only fails when the suite runs as a whole.
func gpsJPEG(t *testing.T, withGPS bool) []byte {
	t.Helper()
	h := 48
	if !withGPS {
		h = 47
	}
	img, err := vips.Black(64, h)
	if err != nil {
		t.Fatalf("Black: %v", err)
	}
	defer img.Close()
	img.SetString("exif-ifd0-Make", "Apple")
	img.SetString("exif-ifd0-Model", "iPhone 17 Pro")
	img.SetString("exif-ifd2-FNumber", "18/10 (1.8)")
	img.SetString("exif-ifd2-ExposureTime", "1/120 (0.008)")
	if withGPS {
		img.SetString("exif-ifd3-GPSLatitude", "31/1 13/1 5799/100")
		img.SetString("exif-ifd3-GPSLatitudeRef", "N")
		img.SetString("exif-ifd3-GPSLongitude", "121/1 28/1 2988/100")
		img.SetString("exif-ifd3-GPSLongitudeRef", "E")
	}
	p := vips.NewJpegExportParams()
	p.Quality = 90
	p.StripMetadata = false
	data, _, err := img.ExportJpeg(p)
	if err != nil {
		t.Fatalf("ExportJpeg: %v", err)
	}
	return data
}

func exifOf(t *testing.T, buf []byte) map[string]string {
	t.Helper()
	img, err := vips.NewImageFromBuffer(buf)
	if err != nil {
		t.Fatalf("解码失败（文件已被破坏？）：%v", err)
	}
	defer img.Close()
	return img.GetExif()
}

func TestStripGPSRemovesLocationKeepsCamera(t *testing.T) {
	src := gpsJPEG(t, true)
	if before := exifOf(t, src); !hasGPSKey(before) {
		t.Fatal("夹具本身就没有 GPS，测不到东西")
	}

	out, changed := StripGPSFromJPEG(src)
	if !changed {
		t.Fatal("有 GPS 却报告未改动")
	}
	if len(out) != len(src) {
		t.Errorf("长度变了：%d → %d；这个改写必须是等长的，否则所有 EXIF 偏移都要重算", len(src), len(out))
	}

	after := exifOf(t, out)
	if hasGPSKey(after) {
		for k, v := range after {
			if strings.Contains(k, "GPS") {
				t.Errorf("定位仍在：%s = %s", k, v)
			}
		}
	}
	// 相机参数必须活下来——这正是用户选“保留原图”的理由
	for _, k := range []string{"exif-ifd0-Make", "exif-ifd0-Model", "exif-ifd2-FNumber", "exif-ifd2-ExposureTime"} {
		if _, ok := after[k]; !ok {
			t.Errorf("%s 被一起删掉了", k)
		}
	}
}

// 坐标本体是 3 个 RATIONAL（24 字节），放不进条目自带的 4 字节，
// 真正的数值另存在别处。只清条目表会把这串数字原样留在文件里。
func TestStripGPSAlsoWipesOutOfLineValues(t *testing.T) {
	src := gpsJPEG(t, true)
	out, changed := StripGPSFromJPEG(src)
	if !changed {
		t.Fatal("未改动")
	}
	// 31°13'57.99" → 分子 5799、分母 100 会以 4 字节小端出现在原图里
	needle := []byte{0xA7, 0x16, 0x00, 0x00} // 5799
	if !bytes.Contains(src, needle) {
		t.Skip("夹具的字节布局与预期不同，跳过")
	}
	if bytes.Contains(out, needle) {
		t.Error("坐标数值仍留在文件中——只清了条目表，没清它指向的数据")
	}
}

func TestStripGPSLeavesCleanFilesUntouched(t *testing.T) {
	src := gpsJPEG(t, false)
	out, changed := StripGPSFromJPEG(src)
	if changed {
		t.Error("没有 GPS 却报告改动了")
	}
	if !bytes.Equal(src, out) {
		t.Error("没有 GPS 时不该动任何字节")
	}
}

func TestStripGPSSurvivesGarbage(t *testing.T) {
	cases := [][]byte{
		nil, {}, {0xFF}, {0xFF, 0xD8}, {0xFF, 0xD8, 0xFF, 0xE1},
		{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x', 'i', 'f', 0, 0},
		[]byte("这不是图片"),
		append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10}, []byte("Exif\x00\x00IIxx\xff\xff\xff\xff")...),
	}
	for i, c := range cases {
		out, changed := StripGPSFromJPEG(c)
		if changed {
			t.Errorf("用例 %d：畸形输入不该报告改动", i)
		}
		if !bytes.Equal(out, c) {
			t.Errorf("用例 %d：畸形输入不该被修改", i)
		}
	}
}

func hasGPSKey(m map[string]string) bool {
	for k := range m {
		if strings.Contains(k, "GPS") {
			return true
		}
	}
	return false
}

// End-to-end: the two paths that store the upload byte-for-byte are the ones
// RemoveMetadata never reaches, so they are the ones that leaked.
func TestOriginalModeDropsGPS(t *testing.T) {
	src := gpsJPEG(t, true)
	res, err := Process(src, Options{Original: true})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	after := exifOf(t, res.Primary.Data)
	if hasGPSKey(after) {
		t.Error("original 模式存下的主图仍带 GPS")
	}
	if _, ok := after["exif-ifd2-FNumber"]; !ok {
		t.Error("光圈被一起删了；保留原图的意义就没了")
	}
}

func TestKeepOriginalVariantDropsGPS(t *testing.T) {
	src := gpsJPEG(t, true)
	res, err := Process(src, Options{KeepOriginal: true, SkipVariants: true})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	var found bool
	for name, v := range res.Variants {
		if !strings.HasPrefix(name, "orig-") {
			continue
		}
		found = true
		if hasGPSKey(exifOf(t, v.Data)) {
			t.Errorf("保留的原图变体 %s 仍带 GPS", name)
		}
	}
	if !found {
		t.Skip("这张图重编码后与原图一致，没有产生 orig- 变体")
	}
}
