import Foundation
import AppKit
import OpenimgKit

/// State for the menu bar app.
///
/// `@MainActor` on the whole thing rather than on individual members: every
/// property here drives a SwiftUI view, and an actor boundary in the middle of
/// that only buys the ability to mutate UI state off the main thread, which is
/// never what you want.
@MainActor
final class AppModel: ObservableObject {
    @Published var server: String = UserDefaults.standard.string(forKey: "server") ?? "https://openimg.io"
    @Published var token: String = ""
    @Published var account: Account?
    @Published var quota: Quota?
    @Published var recent: [RemoteImage] = []
    @Published var status: String = ""
    @Published var busy = false
    @Published var format: LinkFormat = .url
    /// Lives here rather than in a @State on the view: @State is a macro in
    /// current SwiftUI, and its plugin needs a full Xcode install to expand —
    /// with only the Command Line Tools present the whole target stops
    /// compiling. Nothing about this flag needs view-local storage anyway.
    @Published var dropping = false

    private let store = TokenStore()

    var connected: Bool { account != nil }

    init() {
        token = store.load(server: server) ?? ""
    }

    private func client() throws -> OpenimgClient {
        guard let url = URL(string: server.trimmingCharacters(in: .whitespaces)) else {
            throw OpenimgError.badServerURL
        }
        return try OpenimgClient(server: url, token: token)
    }

    // MARK: - Connection

    func connect() async {
        busy = true
        defer { busy = false }
        do {
            let c = try client()
            let me = try await c.me()
            // Only persist once the server has confirmed the token works.
            // Saving on entry leaves a bad credential in the keychain that the
            // next launch silently loads and fails with.
            try store.save(token, server: server)
            UserDefaults.standard.set(server, forKey: "server")
            account = me
            quota = try? await c.quota()
            recent = (try? await c.images(limit: 12).images) ?? []
            status = "已连接 \(me.email)"
        } catch {
            account = nil
            status = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }
    }

    func disconnect() {
        store.delete(server: server)
        token = ""
        account = nil
        quota = nil
        recent = []
        status = "已断开"
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
        busy = true
        defer { busy = false }

        var done = 0
        for url in urls {
            if let reason = rejectLocally(url) {
                status = "\(url.lastPathComponent)：\(reason)"
                continue
            }
            do {
                let c = try client()
                let res = try await c.upload(fileURL: url)
                recent.insert(res.image, at: 0)
                if recent.count > 12 { recent.removeLast(recent.count - 12) }
                done += 1
                copy(format.render(res.image))
                status = res.deduplicated
                    ? "\(res.image.origName) 秒传，链接已复制"
                    : "\(res.image.origName) 已上传，链接已复制"
            } catch {
                status = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
                break // 配额、每日上限、令牌失效——后续文件必然同样失败
            }
        }
        if done > 1 { status = "已上传 \(done) 张，最后一条链接已复制" }
        if done > 0, let c = try? client() { quota = try? await c.quota() }
    }

    /// Rejects what the server would reject anyway.
    ///
    /// Worth doing locally because the daily upload count is consumed by the
    /// attempt, not by the success: finding out from a 415 that HEIC is not in
    /// your tier costs one of the day's fifty either way.
    private func rejectLocally(_ url: URL) -> String? {
        guard let tier = quota?.tier else { return nil }
        let size = (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        if tier.maxFileSize > 0, Int64(size) > tier.maxFileSize {
            return "超过单文件上限 \(bytes(tier.maxFileSize))"
        }
        let ext = url.pathExtension.lowercased()
        let canon = ext == "jpg" ? "jpeg" : (ext == "heif" ? "heic" : ext)
        if !tier.allowedFormats.isEmpty, !tier.allowedFormats.contains(canon) {
            return "你的用户组不支持 \(ext.uppercased())"
        }
        return nil
    }

    func copy(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    func bytes(_ n: Int64) -> String {
        let f = ByteCountFormatter()
        f.countStyle = .binary
        return f.string(fromByteCount: n)
    }
}
