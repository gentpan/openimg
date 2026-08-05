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
    /// SF Symbol for menus. Chosen to read at a glance in a compact picker,
    /// where the label is often truncated.
    public var icon: String {
        switch self {
        case .newest: "arrow.down.to.line"
        case .oldest: "arrow.up.to.line"
        case .largest: "arrow.up.right.square"
        case .smallest: "arrow.down.right.square"
        case .widest: "aspectratio"
        case .name: "textformat.abc"
        }
    }
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
    /// Absolute, and not always on our own origin: an uploaded avatar lives on
    /// the CDN, but one that came from Google or GitHub still points at their
    /// host. Optional because the server omits the key when it is empty.
    public let avatarURL: String?

    /// What to draw when there is no picture. Prefers the nickname over the
    /// address so a letter the user chose wins over one they did not.
    public var initial: String {
        String((name.isEmpty ? email : name).prefix(1)).uppercased()
    }

    enum CodingKeys: String, CodingKey {
        case id, email, name, role
        case avatarURL = "avatar_url"
    }
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

/// `GET /api/storage/summary` — where the quota actually went.
///
/// The three size columns are the interesting part: users see "存储占用" larger
/// than the file they uploaded and have no way to tell whether that is the
/// primary, the WebP/AVIF derivative, or the thumbnails. This is that answer.
public struct StorageSummary: Codable, Sendable {
    public let images: Int
    public let sizeOrig: Int64
    public let sizeStored: Int64
    public let sizePrimary: Int64
    public let sizeVariants: Int64
    public let sizeThumbs: Int64
    public let sizeUnclassified: Int64
    public let byFormat: [FormatSlice]
    public let byProfile: [ProfileSlice]

    public struct FormatSlice: Codable, Sendable, Identifiable {
        public let ext: String
        public let bytes: Int64
        public let images: Int
        public var id: String { ext }
    }
    public struct ProfileSlice: Codable, Sendable, Identifiable {
        public let id: String
        public let name: String
        public let kind: String
        public let bytes: Int64
        public let images: Int
    }

    enum CodingKeys: String, CodingKey {
        case images
        case sizeOrig = "size_orig"
        case sizeStored = "size_stored"
        case sizePrimary = "size_primary"
        case sizeVariants = "size_variants"
        case sizeThumbs = "size_thumbs"
        case sizeUnclassified = "size_unclassified"
        case byFormat = "by_format"
        case byProfile = "by_profile"
    }
}

public struct QuotaTransaction: Codable, Sendable, Identifiable {
    public let id: String
    public let type: String
    public let bytes: Int64
    public let quotaAfter: Int64
    public let usedAfter: Int64
    public let reason: String
    public let createdAt: Date

    /// The ledger mixes capacity grants with consumption, and a chart that
    /// plots them on one axis is meaningless. Grants are positive space you
    /// gained; the rest is space you spent.
    public var isGrant: Bool {
        ["signup_grant", "checkin", "referral", "admin_grant", "delete_refund"].contains(type)
    }
    public var label: String {
        switch type {
        case "signup_grant": "注册赠送"
        case "checkin": "每日签到"
        case "referral": "邀请奖励"
        case "admin_grant": "管理员调整"
        case "upload": "上传"
        case "delete_refund": "删除退还"
        default: type
        }
    }

    enum CodingKeys: String, CodingKey {
        case id, type, bytes, reason
        case quotaAfter = "quota_after"
        case usedAfter = "used_after"
        case createdAt = "created_at"
    }
}

public struct TransactionPage: Codable, Sendable {
    public let transactions: [QuotaTransaction]?
    public let total: Int
}

public struct CheckinRecord: Codable, Sendable, Identifiable {
    public let id: String
    /// yyyy-mm-dd in UTC, as stored.
    public let date: String
    public let bytes: Int64
    public let streak: Int
}

public struct CheckinResult: Codable, Sendable {
    public let grantedBytes: Int64
    public let streak: Int
    public let quotaBytes: Int64
    public let capped: Bool

    enum CodingKeys: String, CodingKey {
        case grantedBytes = "granted_bytes"
        case streak
        case quotaBytes = "quota_bytes"
        case capped
    }
}
