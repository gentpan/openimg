import Foundation
import ImageIO
import UniformTypeIdentifiers
import CoreGraphics

/// Shrinks an image before it is uploaded, when the account asks for a maximum
/// width anyway.
///
/// This is the only processing that happens on this side, and the limits are
/// deliberate. Everything else — compression, derivative formats, metadata
/// stripping — stays on the server, because:
///
///   - Dedup is keyed on the SHA of the *uploaded* bytes. Re-encoding locally
///     makes the same photo hash differently on different machines, so the
///     server stores it twice.
///   - The server re-encodes in optimized mode regardless, so a locally
///     transcoded file would be compressed twice and carry the loss of both.
///   - "Keep original" means the exact bytes the user handed over. Touching
///     them contradicts the setting by definition.
///
/// A downscale escapes all three: the server was going to resample to this
/// width anyway, the format does not change, and it never runs in original
/// mode. What it buys is upload bandwidth — a 6 MB PNG constrained to 1920px
/// leaves as 3.2 MB, and the file that lands on the server is identical to
/// what it would have produced itself.
public enum LocalResize {

    /// Returns a temporary file to upload instead, or nil to send the original.
    ///
    /// Nil covers every "not worth it or not safe" case: no limit set, already
    /// small enough, an animation whose frames would be lost, or any failure
    /// at all. Falling back to the original is always correct — the server
    /// does the same work either way.
    public static func shrink(_ url: URL, maxWidth: Int) -> URL? {
        guard maxWidth > 0 else { return nil }
        guard let src = CGImageSourceCreateWithURL(url as CFURL, nil),
              CGImageSourceGetCount(src) == 1 else { return nil }   // animated: leave alone

        guard let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
              let width = props[kCGImagePropertyPixelWidth] as? Int,
              width > maxWidth else { return nil }

        // The type has to survive the round trip: re-encoding a PNG as JPEG
        // here would be exactly the transcode this is avoiding.
        let writable = (CGImageDestinationCopyTypeIdentifiers() as? [String]) ?? []
        guard let uti = CGImageSourceGetType(src),
              writable.contains(uti as String) else { return nil }

        guard let img = CGImageSourceCreateThumbnailAtIndex(src, 0, [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceThumbnailMaxPixelSize: maxWidth,
            // Honour the EXIF orientation now, since the pixels are being
            // rewritten anyway and the tag will not survive.
            kCGImageSourceCreateThumbnailWithTransform: true,
        ] as CFDictionary) else { return nil }

        let out = FileManager.default.temporaryDirectory
            .appendingPathComponent("openimg-resized-\(UUID().uuidString)")
            .appendingPathExtension(url.pathExtension)

        guard let dst = CGImageDestinationCreateWithURL(out as CFURL, uti, 1, nil) else { return nil }
        // Quality only applies to lossy types; lossless ones ignore it.
        CGImageDestinationAddImage(dst, img, [
            kCGImageDestinationLossyCompressionQuality: 0.92,
        ] as CFDictionary)
        guard CGImageDestinationFinalize(dst) else {
            try? FileManager.default.removeItem(at: out)
            return nil
        }

        // A "shrink" that produced a bigger file is not a shrink. Rare, but it
        // happens with small PNGs and with sources that were already better
        // compressed than what ImageIO writes.
        let before = (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        let after = (try? out.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        if before > 0, after >= before {
            try? FileManager.default.removeItem(at: out)
            return nil
        }
        return out
    }
}
