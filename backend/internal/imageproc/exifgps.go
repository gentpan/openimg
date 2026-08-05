package imageproc

import "encoding/binary"

// StripGPSFromJPEG removes the location tags from a JPEG's EXIF while leaving
// every other byte of the file alone — camera, lens, exposure, and the entropy-
// coded image data all survive bit-for-bit.
//
// This exists because "original" uploads are stored verbatim: Process returns
// the uploaded bytes untouched in that mode, so the RemoveMetadata call earlier
// in the pipeline never reaches them. On the web that means a file the user
// chose to hand over. From a phone's camera roll it means the GPS coordinates
// of wherever the photo was taken, on a publicly addressable URL, by default.
//
// Deleting the whole EXIF block would be far simpler and is the wrong trade:
// aperture and shutter speed are the reason someone picks "keep original" in
// the first place. Only the location goes.
//
// The edit is deliberately size-preserving. Every EXIF offset is relative to
// the start of the TIFF header, so removing bytes would mean rewriting every
// offset in the file and getting one wrong corrupts the image. Instead the GPS
// IFD pointer in IFD0 is blanked in place — same 12 bytes, tag id 0, which
// every reader skips — and the GPS IFD it used to reach is overwritten with
// zeroes. Nothing moves, so nothing else needs fixing up.
//
// Returns the cleaned bytes and whether anything was actually removed. Any
// malformed or unrecognised input returns (buf, false) rather than a guess:
// silently mangling someone's photo is worse than declining to touch it.
func StripGPSFromJPEG(buf []byte) ([]byte, bool) {
	start, length, ok := findExifTIFF(buf)
	if !ok {
		return buf, false
	}
	tiff := buf[start : start+length]

	var bo binary.ByteOrder
	switch {
	case tiff[0] == 'I' && tiff[1] == 'I':
		bo = binary.LittleEndian
	case tiff[0] == 'M' && tiff[1] == 'M':
		bo = binary.BigEndian
	default:
		return buf, false
	}
	if bo.Uint16(tiff[2:4]) != 42 {
		return buf, false
	}

	ifd0 := int(bo.Uint32(tiff[4:8]))
	entry, gpsOff, ok := findGPSPointer(tiff, bo, ifd0)
	if !ok {
		return buf, false
	}

	// Work on a copy so a bail-out halfway through cannot leave the caller
	// holding a half-edited image.
	out := make([]byte, len(buf))
	copy(out, buf)
	t := out[start : start+length] // the same window, into the copy

	zeroIFD(t, bo, gpsOff)
	// Blank the pointer entry: tag 0, type 1 (BYTE), count 0, value 0.
	for i := entry; i < entry+12 && i < len(t); i++ {
		t[i] = 0
	}
	bo.PutUint16(t[entry+2:entry+4], 1)
	return out, true
}

// findExifTIFF locates the TIFF block inside the APP1 "Exif\0\0" segment.
//
// It returns the block's position and length rather than a slice, because the
// caller has to address the same region inside a *copy* of the file. Deriving
// that window from a length alone lands at the end of the file instead of the
// middle, which is silent: detection still reads the right bytes and only the
// writes go astray.
func findExifTIFF(buf []byte) (start, length int, ok bool) {
	if len(buf) < 4 || buf[0] != 0xFF || buf[1] != 0xD8 {
		return 0, 0, false // not a JPEG
	}
	i := 2
	for i+4 <= len(buf) {
		if buf[i] != 0xFF {
			return 0, 0, false // out of sync with the segment chain
		}
		marker := buf[i+1]
		// Standalone markers carry no length; SOS means the entropy-coded
		// scan starts and there is no metadata past here.
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		if marker == 0xDA {
			return 0, 0, false
		}
		segLen := int(binary.BigEndian.Uint16(buf[i+2 : i+4])) // always big-endian
		if segLen < 2 || i+2+segLen > len(buf) {
			return 0, 0, false
		}
		body := buf[i+4 : i+2+segLen]
		if marker == 0xE1 && len(body) >= 14 && string(body[:6]) == "Exif\x00\x00" {
			return i + 4 + 6, len(body) - 6, true
		}
		i += 2 + segLen
	}
	return 0, 0, false
}

// findGPSPointer walks an IFD for tag 0x8825 and returns the offset of that
// 12-byte entry plus the GPS IFD offset it points at.
func findGPSPointer(t []byte, bo binary.ByteOrder, ifd int) (entry, gps int, ok bool) {
	if ifd < 0 || ifd+2 > len(t) {
		return 0, 0, false
	}
	n := int(bo.Uint16(t[ifd : ifd+2]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 0, 0, false
		}
		if bo.Uint16(t[e:e+2]) != 0x8825 {
			continue
		}
		off := int(bo.Uint32(t[e+8 : e+12]))
		if off < 0 || off+2 > len(t) {
			return 0, 0, false // pointer into nowhere; leave the file alone
		}
		return e, off, true
	}
	return 0, 0, false
}

// typeSize is the byte width of each EXIF field type; 0 marks one we do not
// recognise, which makes the caller skip that entry's value rather than
// zeroing a length it guessed at.
var typeSize = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8}

// zeroIFD overwrites an IFD and everything it points at.
//
// The entry table alone is not enough: a GPS coordinate is three RATIONALs, 24
// bytes, far past the 4 bytes an entry can hold inline, so the actual numbers
// live elsewhere in the block and are reached by offset. Blanking the table
// without them would leave the coordinates sitting in the file, unreferenced
// but perfectly readable by anyone who looks.
func zeroIFD(t []byte, bo binary.ByteOrder, ifd int) {
	if ifd+2 > len(t) {
		return
	}
	n := int(bo.Uint16(t[ifd : ifd+2]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			break
		}
		typ := bo.Uint16(t[e+2 : e+4])
		count := int(bo.Uint32(t[e+4 : e+8]))
		if w, known := typeSize[typ]; known {
			if size := w * count; size > 4 {
				off := int(bo.Uint32(t[e+8 : e+12]))
				if off >= 0 && off+size <= len(t) {
					for i := off; i < off+size; i++ {
						t[i] = 0
					}
				}
			}
		}
	}
	end := ifd + 2 + n*12 + 4 // entries plus the next-IFD pointer
	if end > len(t) {
		end = len(t)
	}
	for i := ifd; i < end; i++ {
		t[i] = 0
	}
}
