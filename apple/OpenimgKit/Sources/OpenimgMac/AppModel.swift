import Foundation
import AppKit
import SwiftUI
import OpenimgKit

/// Trailing underscore because `Section` is a SwiftUI type and shadowing it
/// makes every `Section { }` in a List fail to resolve.
enum Section_: String, CaseIterable, Identifiable, Hashable {
    case gallery, upload, settings
    var id: String { rawValue }
    var label: String {
        switch self {
        case .gallery: "图库"
        case .upload: "上传"
        case .settings: "设置"
        }
    }
    var icon: String {
        switch self {
        case .gallery: "square.grid.2x2"
        case .upload: "arrow.up.circle"
        case .settings: "gearshape"
        }
    }
}

/// State for the whole app.
///
/// `@MainActor` on the type rather than on individual members: every property
/// here drives a view, and an actor boundary in the middle of that only buys
/// the ability to mutate UI state off the main thread, which is never wanted.
@MainActor
final class AppModel: ObservableObject {
    static let shared = AppModel()

    // Connection
    @Published var server = UserDefaults.standard.string(forKey: "server") ?? "https://openimg.io"
    @Published var token = ""
    @Published var email = UserDefaults.standard.string(forKey: "email") ?? ""
    @Published var password = ""
    /// Password is the default; the token field stays for self-hosters and for
    /// anyone who would rather not type an account password into a third-party
    /// binary.
    @Published var useToken = false
    @Published var account: Account?
    @Published var quota: Quota?

    // Navigation
    @Published var section: Section_ = .gallery
    @Published var status = ""
    @Published var busy = false

    // Gallery
    @Published var images: [RemoteImage] = []
    @Published var total = 0
    @Published var page = 0
    @Published var pageSize = 25
    @Published var search = ""
    @Published var sort: SortKey = .newest
    @Published var selection: Set<String> = []
    @Published var detail: RemoteImage?
    @Published var linkFormat: LinkFormat = .url

    // Upload
    @Published var uploading = false
    @Published var uploadProgress = ""
    @Published var dropping = false

    /// Every size divides evenly by the five-column grid, so a full page never
    /// ends in a short row with holes where the missing cards would be — the
    /// same reason the web gallery uses these numbers.
    let pageSizes = [25, 50, 100, 200]

    private let store = TokenStore()
    private var searchTask: Task<Void, Never>?

    var connected: Bool { account != nil }
    var pageCount: Int { max(1, Int(ceil(Double(total) / Double(pageSize)))) }

    init() {
        token = store.load(server: server) ?? ""
    }

    func client() throws -> OpenimgClient {
        guard let url = URL(string: server.trimmingCharacters(in: .whitespaces)) else {
            throw OpenimgError.badServerURL
        }
        return try OpenimgClient(server: url, token: token)
    }

    /// Names the credential on the server, so the token list on the website
    /// reads "Openimg for Mac · 西风的 MacBook" instead of an opaque entry the
    /// user cannot place.
    var deviceName: String { Host.current().localizedName ?? "Mac" }

    // MARK: - Connection

    /// Called at launch. Silent on failure — an expired token should land the
    /// user on the settings tab, not greet them with a red banner.
    func restore() async {
        guard !token.isEmpty else { section = .settings; return }
        await connect(quiet: true)
        if !connected { section = .settings }
    }

    /// Exchanges the password for a long-lived token, then connects with it.
    ///
    /// The session the password buys is not what gets stored: it expires in
    /// seven days and the server has no refresh endpoint, so keeping it would
    /// log the user out every week. See OpenimgAuth.signIn.
    func signIn() async {
        busy = true
        defer { busy = false }
        do {
            guard let url = URL(string: server.trimmingCharacters(in: .whitespaces)) else {
                throw OpenimgError.badServerURL
            }
            let (minted, _) = try await OpenimgAuth.signIn(
                server: url, email: email, password: password, device: deviceName
            )
            token = minted
            password = ""      // never held longer than the exchange needs
            UserDefaults.standard.set(email, forKey: "email")
            await connect()
        } catch {
            account = nil
            announce(message(error))
        }
    }

    func connect(quiet: Bool = false) async {
        busy = true
        defer { busy = false }
        do {
            let c = try client()
            let me = try await c.me()

            // Persist only after the server confirms the token, so a typo never
            // lands in the keychain for the next launch to load and fail with.
            // And persistence is not part of connecting: the token is already
            // known good here, so a keychain problem must not report "cannot
            // connect" for a session that works perfectly.
            var warning = ""
            do { try store.save(token, server: server) } catch {
                warning = "（令牌未能保存，下次启动需重新填写）"
            }
            UserDefaults.standard.set(server, forKey: "server")

            account = me
            quota = try? await c.quota()
            await load(resetPage: true)
            if !quiet {
                announce("已连接 \(me.email)\(warning)")
                section = .gallery
            }
        } catch {
            account = nil
            announce(message(error))
        }
    }

    func signOut() {
        store.delete(server: server)
        token = ""
        password = ""
        account = nil
        quota = nil
        images = []
        total = 0
        selection = []
        section = .settings
        // Says what actually happened. A token cannot revoke itself — that is
        // the same boundary that stops a leaked one from taking over an
        // account — so claiming "signed out" would overstate it.
        announce(OpenimgAuth.signOutIsLocalOnly, seconds: 8)
    }

    // MARK: - Gallery

    func load(resetPage: Bool = false) async {
        guard !token.isEmpty else { return }
        if resetPage { page = 0 }
        busy = true
        defer { busy = false }
        do {
            let res = try await client().images(
                limit: pageSize, offset: page * pageSize, query: search, sort: sort
            )
            images = res.images
            total = res.total
            // A checked box on a row that is no longer on screen is a good way
            // to delete the wrong thing.
            selection = []
        } catch {
            announce(message(error))
        }
    }

    /// Typing in the search field should not fire a request per keystroke.
    func searchChanged() {
        searchTask?.cancel()
        searchTask = Task {
            try? await Task.sleep(for: .milliseconds(300))
            guard !Task.isCancelled else { return }
            await load(resetPage: true)
        }
    }

    func go(to newPage: Int) async {
        page = min(max(0, newPage), pageCount - 1)
        await load()
    }

    func toggle(_ id: String) {
        if selection.contains(id) { selection.remove(id) } else { selection.insert(id) }
    }

    func toggleAll() {
        if selection.count == images.count { selection = [] }
        else { selection = Set(images.map(\.id)) }
    }

    func deleteSelected() async {
        let ids = Array(selection)
        guard !ids.isEmpty else { return }

        let freed = images.filter { selection.contains($0.id) }.reduce(Int64(0)) { $0 + $1.sizeStored }
        let alert = NSAlert()
        alert.messageText = "删除 \(ids.count) 张图片？"
        alert.informativeText = "对象会被清除，占用的 \(Self.bytes(freed)) 退还到你的空间。此操作不可撤销。"
        alert.alertStyle = .warning
        alert.addButton(withTitle: "删除")
        alert.addButton(withTitle: "取消")
        guard alert.runModal() == .alertFirstButtonReturn else { return }

        busy = true
        defer { busy = false }
        do {
            // The server caps one call at 500.
            var deleted = 0
            for start in stride(from: 0, to: ids.count, by: 500) {
                let chunk = Array(ids[start..<min(start + 500, ids.count)])
                deleted += try await client().bulkDelete(ids: chunk).deleted
            }
            announce("已删除 \(deleted) 张")
            detail = nil
            quota = try? await client().quota()
            await load()
        } catch {
            announce(message(error))
        }
    }

    func delete(_ img: RemoteImage) async {
        selection = [img.id]
        await deleteSelected()
    }

    // MARK: - Upload

    func pickAndUpload() async {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = true
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.image]
        guard panel.runModal() == .OK else { return }
        await upload(panel.urls)
    }

    func upload(_ urls: [URL]) async {
        guard !urls.isEmpty else { return }
        uploading = true
        defer { uploading = false; uploadProgress = "" }

        var done = 0
        var lastLink = ""
        for (i, url) in urls.enumerated() {
            uploadProgress = "\(i + 1)/\(urls.count) \(url.lastPathComponent)"
            if let reason = rejectLocally(url) {
                announce("\(url.lastPathComponent)：\(reason)")
                continue
            }
            do {
                let res = try await client().upload(fileURL: url)
                lastLink = linkFormat.render(res.image)
                done += 1
            } catch {
                announce(message(error))
                break // 配额、每日上限、令牌失效——后续文件必然同样失败
            }
        }
        if done > 0 {
            copy(lastLink)
            status = done == 1 ? "已上传，链接已复制" : "已上传 \(done) 张，最后一条链接已复制"
            quota = try? await client().quota()
            await load(resetPage: true)
            section = .gallery
        }
    }

    /// Rejects what the server would reject anyway.
    ///
    /// Worth doing locally because the daily upload count is consumed by the
    /// attempt, not by the success: learning from a 415 that HEIC is not in
    /// your tier costs one of the day's allowance either way.
    private func rejectLocally(_ url: URL) -> String? {
        guard let tier = quota?.tier else { return nil }
        let size = (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        if tier.maxFileSize > 0, Int64(size) > tier.maxFileSize {
            return "超过单文件上限 \(Self.bytes(tier.maxFileSize))"
        }
        let ext = url.pathExtension.lowercased()
        let canon = ext == "jpg" ? "jpeg" : (ext == "heif" ? "heic" : ext)
        if !tier.allowedFormats.isEmpty, !tier.allowedFormats.contains(canon) {
            return "你的用户组不支持 \(ext.uppercased())"
        }
        return nil
    }

    // MARK: - Helpers

    func copy(_ text: String) {
        guard !text.isEmpty else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    private var toastTask: Task<Void, Never>?

    /// Shows a message and clears it on its own. A status line that never goes
    /// away turns into furniture and stops being read.
    func announce(_ text: String, seconds: Double = 4) {
        withAnimation(.spring(duration: 0.3)) { status = text }
        toastTask?.cancel()
        toastTask = Task {
            try? await Task.sleep(for: .seconds(seconds))
            guard !Task.isCancelled else { return }
            withAnimation(.easeOut(duration: 0.25)) { status = "" }
        }
    }

    func message(_ error: Error) -> String {
        (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
    }

    static func bytes(_ n: Int64) -> String {
        let f = ByteCountFormatter()
        f.countStyle = .binary
        return f.string(fromByteCount: n)
    }
    func bytes(_ n: Int64) -> String { Self.bytes(n) }
}
