import Foundation

/// The upload/list payload, mirroring `imageOut` in
/// `backend/internal/api/image_handlers.go`.
///
/// Only the fields a client acts on are declared. `imageOut` embeds the whole
/// `models.Image`, so it carries a good deal more — object keys, per-tier byte
/// counts, backup state — and decoding all of it would turn every backend
/// column addition into a client change.
public struct RemoteImage: Codable, Sendable, Identifiable, Hashable {
    public let id: String
    public let origName: String
    public let ext: String
    public let width: Int
    public let height: Int
    public let sizeStored: Int64
    public let url: String
    public let thumbURL: String
    public let shortURL: String?
    public let markdown: String
    public let html: String
    public let bbcode: String
    public let createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case origName = "orig_name"
        case ext, width, height
        case sizeStored = "size_stored"
        case url
        case thumbURL = "thumb_url"
        case shortURL = "short_url"
        case markdown, html, bbcode
        case createdAt = "created_at"
    }
}

public struct UploadResponse: Codable, Sendable {
    public let image: RemoteImage
    /// True when the server already held these exact bytes and charged nothing.
    public let deduplicated: Bool
}

/// Mirrors SORTS in frontend/src/api.ts; the server validates against the same
/// whitelist and silently falls back to newest for anything else.
public enum SortKey: String, CaseIterable, Sendable, Identifiable {
    case newest, oldest, largest, smallest, widest, name
    public var id: String { rawValue }
    public var label: String {
        switch self {
        case .newest: "最新上传"
        case .oldest: "最早上传"
        case .largest: "占用最大"
        case .smallest: "占用最小"
        case .widest: "分辨率最高"
        case .name: "文件名"
        }
    }
}

public struct BulkDeleteResult: Codable, Sendable {
    public let deleted: Int
    public let remaining: Int
}

public struct ImagePage: Codable, Sendable {
    public let images: [RemoteImage]
    public let total: Int
    public let limit: Int
    public let offset: Int
}

/// What `GET /api/quota` reports. The tier block is the useful half: it is what
/// lets a client reject a file locally instead of spending one of the day's
/// uploads discovering the limit from a 413.
public struct Quota: Codable, Sendable {
    public let quotaBytes: Int64
    public let usedBytes: Int64
    public let availableBytes: Int64
    public let imageCount: Int
    public let uploadsToday: Int
    public let tier: Tier

    public struct Tier: Codable, Sendable {
        public let name: String
        public let maxFileSize: Int64
        public let dailyUploadCount: Int
        public let allowedFormats: [String]

        enum CodingKeys: String, CodingKey {
            case name
            case maxFileSize = "max_file_size"
            case dailyUploadCount = "daily_upload_count"
            case allowedFormats = "allowed_formats"
        }
    }

    enum CodingKeys: String, CodingKey {
        case quotaBytes = "quota_bytes"
        case usedBytes = "used_bytes"
        case availableBytes = "available_bytes"
        case imageCount = "image_count"
        case uploadsToday = "uploads_today"
        case tier
    }
}

public struct Account: Codable, Sendable {
    public let id: String
    public let email: String
    public let name: String
    public let role: String
}

/// The link shapes the web UI offers, so a menu bar app can put the same set
/// behind one picker instead of inventing its own formatting.
public enum LinkFormat: String, CaseIterable, Sendable {
    case url, markdown, html, bbcode

    public var label: String {
        switch self {
        case .url: "直链"
        case .markdown: "Markdown"
        case .html: "HTML"
        case .bbcode: "BBCode"
        }
    }

    public func render(_ image: RemoteImage) -> String {
        switch self {
        case .url: image.url
        case .markdown: image.markdown
        case .html: image.html
        case .bbcode: image.bbcode
        }
    }
}
