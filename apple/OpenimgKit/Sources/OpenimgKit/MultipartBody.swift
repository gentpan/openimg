import Foundation

/// Builds a multipart/form-data body on disk rather than in memory.
///
/// Two reasons, and the second is the one that forces it. A 50 MB upload held
/// as Data costs 50 MB of resident memory per concurrent upload, which a menu
/// bar process should not do. More importantly `URLSession`'s background
/// configuration only accepts `uploadTask(with:fromFile:)` — hand it a body in
/// memory and the upload dies with the app instead of surviving a quit, a lid
/// close, or a reboot.
///
/// The caller owns the returned file. When the upload is a background task it
/// must outlive the process, because the system re-reads it on retry: deleting
/// it in a `defer` next to the request is correct for a foreground upload and
/// silently breaks retries for a background one.
public enum MultipartBody {
    public static func write(
        fileURL: URL,
        fieldName: String,
        filename: String,
        boundary: String,
        directory: URL? = nil
    ) throws -> URL {
        let dir = directory ?? FileManager.default.temporaryDirectory
        let out = dir.appendingPathComponent("openimg-upload-\(UUID().uuidString).multipart")

        FileManager.default.createFile(atPath: out.path, contents: nil)
        let handle = try FileHandle(forWritingTo: out)
        defer { try? handle.close() }

        try handle.write(contentsOf: Data(header(
            fieldName: fieldName, filename: filename, boundary: boundary
        ).utf8))

        // Streamed in chunks so a large file never lands in memory whole.
        let input = try FileHandle(forReadingFrom: fileURL)
        defer { try? input.close() }
        while true {
            let chunk = try input.read(upToCount: 1 << 20) ?? Data()
            if chunk.isEmpty { break }
            try handle.write(contentsOf: chunk)
        }

        try handle.write(contentsOf: Data("\r\n--\(boundary)--\r\n".utf8))
        return out
    }

    static func header(fieldName: String, filename: String, boundary: String) -> String {
        """
        --\(boundary)\r
        Content-Disposition: form-data; name="\(fieldName)"; filename="\(escape(filename))"\r
        Content-Type: \(mimeType(for: filename))\r
        \r

        """
    }

    /// A filename carrying a quote or a newline would otherwise break out of
    /// the Content-Disposition header and let the caller forge parts.
    static func escape(_ name: String) -> String {
        name
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
            .replacingOccurrences(of: "\r", with: "")
            .replacingOccurrences(of: "\n", with: "")
    }

    static func mimeType(for filename: String) -> String {
        switch (filename as NSString).pathExtension.lowercased() {
        case "jpg", "jpeg": "image/jpeg"
        case "png": "image/png"
        case "gif": "image/gif"
        case "webp": "image/webp"
        case "avif": "image/avif"
        case "heic", "heif": "image/heic"
        case "bmp": "image/bmp"
        case "tif", "tiff": "image/tiff"
        default: "application/octet-stream"
        }
    }
}
