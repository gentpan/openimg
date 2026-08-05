import Foundation

public enum OpenimgError: Error, LocalizedError, Equatable {
    case badServerURL
    case insecureServer(host: String)
    case unauthorized(String)
    case quotaExhausted(String)
    case dailyLimitReached(used: Int, limit: Int)
    case rateLimited(retryAfter: Int)
    case rejected(status: Int, message: String)
    case transport(String)

    public var errorDescription: String? {
        switch self {
        case .badServerURL: "服务器地址无法解析"
        case .insecureServer(let host): "\(host) 不是 https，令牌会以明文发送"
        case .unauthorized(let m): "令牌无效：\(m)"
        case .quotaExhausted(let m): m
        case .dailyLimitReached(let used, let limit): "今日上传已达上限（\(used)/\(limit)）"
        case .rateLimited(let s): "上传过于频繁，请 \(s) 秒后再试"
        case .rejected(_, let m): m
        case .transport(let m): m
        }
    }
}

/// Talks to one openimg instance with a personal access token.
///
/// The server address is a parameter, not a constant: this is an MIT project
/// people self-host, and a client hard-wired to openimg.io is useless to them.
/// That is also why `init` refuses plain http for anything but a loopback host
/// — the token travels in a header on every request, and a typo'd hostname
/// would otherwise hand it to whoever answers.
public struct OpenimgClient: Sendable {
    public let server: URL
    private let token: String
    private let session: URLSession

    public init(server: URL, token: String, session: URLSession = .shared) throws {
        guard let host = server.host, !host.isEmpty else { throw OpenimgError.badServerURL }
        let loopback = host == "localhost" || host == "127.0.0.1" || host == "::1"
        if server.scheme != "https" && !loopback {
            throw OpenimgError.insecureServer(host: host)
        }
        self.server = server
        self.token = token
        self.session = session
    }

    // MARK: - Requests

    private func request(_ method: String, _ path: String, query: [URLQueryItem] = []) -> URLRequest {
        var comps = URLComponents(url: server.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { comps.queryItems = query }
        var req = URLRequest(url: comps.url!)
        req.httpMethod = method
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        return req
    }

    private func decode<T: Decodable>(_ type: T.Type, _ data: Data, _ response: URLResponse) throws -> T {
        guard let http = response as? HTTPURLResponse else {
            throw OpenimgError.transport("没有 HTTP 响应")
        }
        guard (200..<300).contains(http.statusCode) else {
            throw Self.failure(status: http.statusCode, body: data, headers: http)
        }
        let dec = JSONDecoder()
        dec.dateDecodingStrategy = .iso8601WithFractionalSeconds
        do {
            return try dec.decode(T.self, from: data)
        } catch {
            throw OpenimgError.transport("响应解析失败：\(error.localizedDescription)")
        }
    }

    /// Maps a failed response onto something a UI can act on.
    ///
    /// The distinctions matter more than they look. 429 arrives for two
    /// unrelated reasons: the per-minute rate limit, which clears on its own,
    /// and the daily upload count, which does not clear until tomorrow. Telling
    /// the user to "try again shortly" in the second case is simply wrong, and
    /// a screenshot-uploading tool hits the daily cap far more often than the
    /// rate limit.
    static func failure(status: Int, body: Data, headers: HTTPURLResponse) -> OpenimgError {
        let obj = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] ?? [:]
        let message = obj["error"] as? String ?? "服务器返回 \(status)"

        switch status {
        case 401:
            return .unauthorized(message)
        case 429:
            if let used = obj["used"] as? Int, let limit = obj["limit"] as? Int {
                return .dailyLimitReached(used: used, limit: limit)
            }
            let retry = obj["retry_after"] as? Int
                ?? Int(headers.value(forHTTPHeaderField: "Retry-After") ?? "") ?? 60
            return .rateLimited(retryAfter: retry)
        case 507:
            return .quotaExhausted(message)
        default:
            return .rejected(status: status, message: message)
        }
    }

    // MARK: - Endpoints

    public func me() async throws -> Account {
        let (data, resp) = try await session.data(for: request("GET", "auth/me"))
        return try decode(Account.self, data, resp)
    }

    public func quota() async throws -> Quota {
        let (data, resp) = try await session.data(for: request("GET", "api/quota"))
        return try decode(Quota.self, data, resp)
    }

    /// Only the fields passed are changed; the server ignores the rest.
    public func updatePreferences(
        uploadMode: UploadMode? = nil,
        variantFormat: VariantFormat? = nil,
        maxImageWidth: Int? = nil
    ) async throws {
        var body: [String: Any] = [:]
        if let uploadMode { body["upload_mode"] = uploadMode.rawValue }
        if let variantFormat { body["variant_format"] = variantFormat.rawValue }
        if let maxImageWidth { body["max_image_width"] = maxImageWidth }
        guard !body.isEmpty else { return }

        var req = request("PATCH", "api/preferences")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: body)
        let (data, resp) = try await session.data(for: req)
        _ = try decode(OK.self, data, resp)
    }

    public func storageSummary() async throws -> StorageSummary {
        let (data, resp) = try await session.data(for: request("GET", "api/storage/summary"))
        return try decode(StorageSummary.self, data, resp)
    }

    public func transactions(limit: Int = 50, offset: Int = 0) async throws -> TransactionPage {
        let req = request("GET", "api/quota/transactions", query: [
            .init(name: "limit", value: String(limit)),
            .init(name: "offset", value: String(offset)),
        ])
        let (data, resp) = try await session.data(for: req)
        return try decode(TransactionPage.self, data, resp)
    }

    public func checkinHistory(limit: Int = 120) async throws -> [CheckinRecord] {
        let req = request("GET", "api/checkin/history", query: [.init(name: "limit", value: String(limit))])
        let (data, resp) = try await session.data(for: req)
        struct Wrap: Decodable { let records: [CheckinRecord]? }
        return try decode(Wrap.self, data, resp).records ?? []
    }

    /// 409 means already checked in today, which is a normal outcome rather
    /// than an error worth surfacing as one.
    public func checkin() async throws -> CheckinResult {
        let (data, resp) = try await session.data(for: request("POST", "api/checkin"))
        return try decode(CheckinResult.self, data, resp)
    }

    public func images(
        limit: Int = 25,
        offset: Int = 0,
        query search: String = "",
        sort: SortKey = .newest
    ) async throws -> ImagePage {
        var q: [URLQueryItem] = [
            .init(name: "limit", value: String(limit)),
            .init(name: "offset", value: String(offset)),
            .init(name: "sort", value: sort.rawValue),
        ]
        // Omitted rather than sent empty: the server treats a present `q` as a
        // filter and would ILIKE against '%%'.
        let trimmed = search.trimmingCharacters(in: .whitespaces)
        if !trimmed.isEmpty { q.append(.init(name: "q", value: trimmed)) }

        let (data, resp) = try await session.data(for: request("GET", "api/images", query: q))
        return try decode(ImagePage.self, data, resp)
    }

    /// Deletes by id. The server caps a single call at 500; callers with more
    /// than that must chunk, which is why this takes an array and not a set.
    public func bulkDelete(ids: [String]) async throws -> BulkDeleteResult {
        var req = request("POST", "api/images/bulk-delete")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["ids": ids])
        let (data, resp) = try await session.data(for: req)
        return try decode(BulkDeleteResult.self, data, resp)
    }

    /// Fetches raw bytes for a thumbnail or image URL.
    ///
    /// Object URLs point at the CDN, not at the API, and carry no credentials —
    /// so this deliberately does not attach the token. Sending it to whatever
    /// host `public_base_url` names would leak it to a third party the moment
    /// someone points a storage profile at one.
    public func fetchData(_ urlString: String) async throws -> Data {
        guard let url = URL(string: urlString) else { throw OpenimgError.badServerURL }
        let (data, resp) = try await session.data(from: url)
        guard let http = resp as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw OpenimgError.transport("图片加载失败")
        }
        return data
    }

    public func delete(id: String) async throws {
        let (data, resp) = try await session.data(for: request("DELETE", "api/images/\(id)"))
        _ = try decode(OK.self, data, resp)
    }

    /// Uploads from a file on disk.
    ///
    /// `fromFile` rather than `httpBody` because that is the only form a
    /// background URLSession accepts, and a menu bar app that drops its uploads
    /// when the user closes the lid is not much of a menu bar app. The multipart
    /// body is assembled on disk for the same reason.
    public func upload(
        fileURL: URL,
        filename: String? = nil,
        onProgress: (@Sendable (Double) -> Void)? = nil
    ) async throws -> UploadResponse {
        let boundary = "openimg.\(UUID().uuidString)"
        let body = try MultipartBody.write(
            fileURL: fileURL,
            fieldName: "file", // fixed server-side: c.FormFile("file")
            filename: filename ?? fileURL.lastPathComponent,
            boundary: boundary
        )
        defer { try? FileManager.default.removeItem(at: body) }

        var req = request("POST", "api/upload")
        req.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        // A delegate, because `upload(for:fromFile:)` reports nothing until it
        // finishes. On a 20 MB file over a slow link that is a UI frozen on
        // "uploading…" for a minute with no way to tell it apart from a hang.
        let (data, resp) = try await session.upload(
            for: req, fromFile: body,
            delegate: onProgress.map(UploadProgressDelegate.init)
        )
        return try decode(UploadResponse.self, data, resp)
    }

    private struct OK: Decodable { let ok: Bool }
}

extension JSONDecoder.DateDecodingStrategy {
    /// Go's time.Time marshals with nanoseconds, which `.iso8601` rejects
    /// outright — the whole response fails to decode over a fractional second.
    static let iso8601WithFractionalSeconds = custom { decoder in
        let raw = try decoder.singleValueContainer().decode(String.self)
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let d = withFraction.date(from: raw) { return d }
        let plain = ISO8601DateFormatter()
        plain.formatOptions = [.withInternetDateTime]
        if let d = plain.date(from: raw) { return d }
        throw DecodingError.dataCorruptedError(
            in: try decoder.singleValueContainer(),
            debugDescription: "无法解析时间：\(raw)"
        )
    }
}


/// Reports upload progress. Separate from the client because URLSession keeps a
/// strong reference to a task delegate for the life of the task, and hanging
/// that off a value type would be a retain cycle waiting to happen.
final class UploadProgressDelegate: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
    private let onProgress: @Sendable (Double) -> Void
    init(_ onProgress: @escaping @Sendable (Double) -> Void) { self.onProgress = onProgress }

    func urlSession(_ session: URLSession, task: URLSessionTask,
                    didSendBodyData bytesSent: Int64,
                    totalBytesSent: Int64,
                    totalBytesExpectedToSend: Int64) {
        guard totalBytesExpectedToSend > 0 else { return }
        onProgress(Double(totalBytesSent) / Double(totalBytesExpectedToSend))
    }
}
