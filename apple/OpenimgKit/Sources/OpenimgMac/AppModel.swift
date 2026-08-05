import Foundation
import AppKit
import SwiftUI
import OpenimgKit

/// Trailing underscore because `Section` is a SwiftUI type and shadowing it
/// makes every `Section { }` in a List fail to resolve.
enum Section_: String, CaseIterable, Identifiable, Hashable {
    case overview, gallery, upload, settings
    var id: String { rawValue }
    var label: String {
        switch self {
        case .overview: "概览"
        case .gallery: "图库"
        case .upload: "上传"
        case .settings: "设置"
        }
    }
    var icon: String {
        switch self {
        case .overview: "chart.bar.doc.horizontal"
        case .gallery: "square.grid.2x2"
        case .upload: "arrow.up.circle"
        case .settings: "gearshape"
        }
    }

    /// The filled counterpart, used for the selected row. Swapping between the
    /// two is what `.symbolEffect(.replace)` animates — an outline morphing
    /// into a solid reads as "this one is on" without any extra chrome.
    var iconFilled: String {
        switch self {
        case .overview: "chart.bar.doc.horizontal.fill"
        case .gallery: "square.grid.2x2.fill"
        case .upload: "arrow.up.circle.fill"
        case .settings: "gearshape.fill"
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
    /// Fixed, and not shown anywhere in the UI. The Kit still takes a URL —
    /// it is a library and a hard-wired host would make it untestable — but
    /// this app targets one service, so putting an address field in front of
    /// the user only invites them to type something that will not work.
    let server = "https://openimg.io"
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

    // Stats
    @Published var summary: StorageSummary?
    @Published var transactions: [QuotaTransaction] = []
    @Published var checkins: [CheckinRecord] = []
    @Published var statsLoading = false

    // Upload
    @Published var uploading = false
    @Published var queue: [UploadItem] = []
    @Published var dropping = false

    /// Overall progress across the batch, weighted by file size — a 40 KB icon
    /// finishing should not move the bar as far as a 20 MB photo.
    ///
    /// Falls back to counting files when no size is known. Reading a file size
    /// can fail, and a weighted average over all-zero weights is 0/0: the bar
    /// would sit at empty with half the batch already uploaded.
    var batchProgress: Double {
        guard !queue.isEmpty else { return 0 }
        let total = queue.reduce(0.0) { $0 + Double($1.size) }
        if total <= 0 {
            return queue.reduce(0.0) { $0 + $1.fraction } / Double(queue.count)
        }
        return queue.reduce(0.0) { $0 + Double($1.size) * $1.fraction } / total
    }

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

    private let oauth = OAuthSignIn()

    var canSubmit: Bool {
        useToken ? !token.isEmpty : (!email.isEmpty && !password.isEmpty)
    }

    /// What the primary button and the return key both do, so the form has one
    /// submit path regardless of which mode it is in.
    func primaryAction() async {
        guard canSubmit, !busy else { return }
        if useToken { await connect() } else { await signIn() }
    }

    /// Google, GitHub or passkey. All three finish the same way: the sheet
    /// hands back a one-time code and the token comes from trading it, so
    /// nothing long-lived ever travels through a URL.
    ///
    /// The providers go straight to their own start route. Passkey cannot —
    /// WebAuthn runs in the page, and driving it natively needs an
    /// associated-domains entitlement, which needs a Team ID a locally signed
    /// build does not have. It opens the site's own sign-in page instead,
    /// which hands back a code through the same handoff.
    func signIn(with method: SignInMethod) async {
        busy = true
        defer { busy = false }
        do {
            let code: String?
            switch method {
            case .passkey:
                code = try await oauth.startWebLogin(server: server)
            case .google, .github:
                code = try await oauth.start(provider: method.rawValue, server: server)
            }
            guard let code else { return } // 用户关掉了登录窗口
            guard let url = URL(string: server) else { throw OpenimgError.badServerURL }
            let (minted, _) = try await OpenimgAuth.exchange(
                server: url, code: code, device: deviceName
            )
            token = minted
            await connect()
        } catch {
            account = nil
            announce(message(error))
        }
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

            account = me
            quota = try? await c.quota()
            await load(resetPage: true)
            await loadStats()
            if !quiet {
                announce("已连接 \(me.email)\(warning)")
                section = .overview
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

    // MARK: - Stats

    var streak: Int { checkins.first?.streak ?? 0 }

    /// The server records days as UTC `yyyy-mm-dd`, so "today" has to be asked
    /// in UTC too — comparing against a local date puts the app a day out for
    /// anyone east of Greenwich, which is most of its users.
    var checkedInToday: Bool {
        var cal = Calendar(identifier: .gregorian)
        cal.timeZone = TimeZone(identifier: "UTC")!
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.timeZone = cal.timeZone
        return checkins.contains { $0.date == f.string(from: Date()) }
    }

    /// Monday-first, because that is how the streak reads to a user even
    /// though the server keys days in UTC.
    var thisWeek: [CheckinDay] {
        var cal = Calendar(identifier: .gregorian)
        cal.timeZone = TimeZone(identifier: "UTC")!
        cal.firstWeekday = 2
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.timeZone = cal.timeZone

        let now = Date()
        let today = f.string(from: now)
        let claimed = Set(checkins.map(\.date))
        guard let start = cal.dateInterval(of: .weekOfYear, for: now)?.start else { return [] }

        return (0..<7).compactMap { i in
            guard let day = cal.date(byAdding: .day, value: i, to: start) else { return nil }
            let key = f.string(from: day)
            return CheckinDay(
                id: key,
                label: ["一", "二", "三", "四", "五", "六", "日"][i],
                claimed: claimed.contains(key),
                isToday: key == today,
                future: key > today
            )
        }
    }

    /// How far into the current month-long milestone the streak has come.
    var monthProgress: (done: Int, total: Int) {
        let total = quota?.checkin?.daysPerMonth ?? 30
        guard total > 0 else { return (0, 30) }
        return (streak % total == 0 && streak > 0 ? total : streak % total, total)
    }

    func loadStats() async {
        guard connected else { return }
        statsLoading = true
        defer { statsLoading = false }
        guard let c = try? client() else { return }
        // Independent panels: one failing endpoint should blank its own card,
        // not the whole page.
        async let sum = try? await c.storageSummary()
        async let tx = try? await c.transactions(limit: 50)
        async let ck = try? await c.checkinHistory(limit: 130)
        summary = await sum
        transactions = await tx?.transactions ?? []
        checkins = await ck ?? []
    }

    func checkin() async {
        busy = true
        defer { busy = false }
        do {
            let r = try await client().checkin()
            announce(r.capped
                ? "签到成功，但空间已达上限，本次未发放"
                : "签到成功，+\(Self.bytes(r.grantedBytes))，连续 \(r.streak) 天")
            quota = try? await client().quota()
            await loadStats()
        } catch {
            announce(message(error))
        }
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

    /// What the refresh control and ⌘R do, which depends on the page: the
    /// gallery reloads its page, the overview reloads its numbers.
    func refreshCurrent() async {
        switch section {
        case .gallery: await load()
        case .overview: await loadStats()
        case .upload, .settings:
            quota = try? await client().quota()
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
        defer { uploading = false }

        // The whole batch goes on screen up front, so the user can see what was
        // accepted before anything starts moving.
        queue = urls.map { UploadItem(url: $0) }

        var done = 0
        var lastLink = ""
        for i in queue.indices {
            let url = queue[i].url
            if let reason = rejectLocally(url) {
                queue[i].state = .failed(reason)
                continue
            }
            queue[i].state = .uploading
            do {
                let id = queue[i].id
                let res = try await client().upload(fileURL: url) { [weak self] p in
                    Task { @MainActor in
                        guard let self, let k = self.queue.firstIndex(where: { $0.id == id })
                        else { return }
                        self.queue[k].progress = p
                    }
                }
                queue[i].state = .done
                queue[i].progress = 1
                queue[i].deduplicated = res.deduplicated
                lastLink = linkFormat.render(res.image)
                done += 1
            } catch {
                queue[i].state = .failed(message(error))
                // Quota, daily cap and an invalid token all doom the rest of
                // the batch, so mark them rather than retrying into the wall.
                for j in queue.indices where j > i && queue[j].state == .queued {
                    queue[j].state = .failed("已取消")
                }
                announce(message(error))
                break
            }
        }
        if done > 0 {
            copy(lastLink)
            announce(done == 1 ? "已上传，链接已复制" : "已上传 \(done) 张，最后一条链接已复制")
            quota = try? await client().quota()
            await load(resetPage: true)
        }
    }

    func clearQueue() { queue.removeAll() }

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
        // Otherwise a zero-byte file reads as "Zero KB", which is both wrong
        // and the sort of string that makes a UI look unfinished.
        f.allowsNonnumericFormatting = false
        return f.string(fromByteCount: n)
    }
    func bytes(_ n: Int64) -> String { Self.bytes(n) }
}
