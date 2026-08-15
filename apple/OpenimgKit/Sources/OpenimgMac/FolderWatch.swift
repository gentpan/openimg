import Foundation
import CoreServices
import CryptoKit
import OpenimgKit

/// FSEvents 流:递归监听多个目录。事件只用来说"有动静",丢了细节无所谓——
/// 扫描永远是全量对账(清单快路径让它对已收录文件几乎零成本),事件流的
/// 作用只是省掉轮询。
final class FolderEventStream {
    private var stream: FSEventStreamRef?
    private let box: Box

    private final class Box {
        let fire: @Sendable () -> Void
        init(_ f: @escaping @Sendable () -> Void) { fire = f }
    }

    init?(paths: [String], queue: DispatchQueue, onChange: @escaping @Sendable () -> Void) {
        guard !paths.isEmpty else { return nil }
        box = Box(onChange)
        let cb: FSEventStreamCallback = { _, info, _, _, _, _ in
            guard let info else { return }
            Unmanaged<Box>.fromOpaque(info).takeUnretainedValue().fire()
        }
        // retain/release 必须填:回调跑在私有队列上,stop() 在主线程释放本
        // 实例连同 Box——没有这对回调,在途回调的 takeUnretainedValue 会摸
        // 到已释放内存(FSEvents 提供这两个字段正是为了这个窗口)。
        var ctx = FSEventStreamContext(
            version: 0,
            info: Unmanaged.passUnretained(box).toOpaque(),
            retain: { info in
                guard let info else { return nil }
                return UnsafeRawPointer(Unmanaged<Box>.fromOpaque(info).retain().toOpaque())
            },
            release: { info in
                guard let info else { return }
                Unmanaged<Box>.fromOpaque(info).release()
            },
            copyDescription: nil)
        guard let s = FSEventStreamCreate(nil, cb, &ctx,
                                          paths as CFArray,
                                          FSEventStreamEventId(kFSEventStreamEventIdSinceNow),
                                          2.0,
                                          FSEventStreamCreateFlags(kFSEventStreamCreateFlagNoDefer)) else {
            return nil
        }
        stream = s
        FSEventStreamSetDispatchQueue(s, queue)
        FSEventStreamStart(s)
    }

    func stop() {
        guard let s = stream else { return }
        FSEventStreamStop(s)
        FSEventStreamInvalidate(s)
        FSEventStreamRelease(s)
        stream = nil
    }

    deinit { stop() }
}

/// 目录监控自动上传。
///
/// 设计边界(和网盘划清界限):只增不删——本地删除不动云端(外链持久有效
/// 是产品承诺),云端删除也不回头删本地。这不是同步,是自动上传。
extension AppModel {
    /// 扫描候选的扩展名粗筛。逐文件的组级校验(单文件大小/组允许格式)
    /// 仍走 rejectLocally,这里只是不给非图片文件进入流水线的机会。
    nonisolated static let watchExts: Set<String> = [
        "jpg", "jpeg", "jpe", "png", "gif", "webp", "avif",
        "heic", "heif", "bmp", "tif", "tiff",
    ]

    /// 清单按 服务器+账号 键控:换账号/换自建服务器后,旧账号的上传记录
    /// 对新账号毫无意义——沿用会让文件被误判"已收录"而永不上传。
    private var watchManifestURL: URL {
        let key = "\(server)|\(account?.id ?? "")"
        let hash = SHA256.hash(data: Data(key.utf8))
            .prefix(8).map { String(format: "%02x", $0) }.joined()
        let base = FileManager.default.urls(for: .applicationSupportDirectory,
                                            in: .userDomainMask)[0]
        return base.appendingPathComponent("io.openimg.mac/watch-manifest-\(hash).json")
    }

    /// 连接成功、目录/开关变化后都走这里:重建事件流并安排一次全量扫描。
    func watchSetup() {
        watchStream?.stop()
        watchStream = nil
        guard watchEnabled, connected, !watchFolders.isEmpty else { return }
        let q = DispatchQueue(label: "io.openimg.mac.fsevents")
        watchStream = FolderEventStream(paths: watchFolders, queue: q) { [weak self] in
            Task { @MainActor in self?.watchScanSoon() }
        }
        watchScanSoon(after: 0.5)
    }

    func watchStop() {
        watchStream?.stop()
        watchStream = nil
        watchRescan?.cancel()
        watchRescan = nil
    }

    func watchSetEnabled(_ on: Bool) {
        watchEnabled = on
        UserDefaults.standard.set(on, forKey: "watchEnabled")
        watchPausedReason = nil
        watchSkip.removeAll()
        watchLastIssue = nil
        if on {
            watchSetup()
        } else {
            watchStop()
            watchStatus = ""
        }
    }

    func watchAddFolder(_ path: String) {
        guard !watchFolders.contains(path) else { return }
        watchFolders.append(path)
        UserDefaults.standard.set(watchFolders, forKey: "watchFolders")
        watchSetup()
    }

    func watchRemoveFolder(_ path: String) {
        watchFolders.removeAll { $0 == path }
        UserDefaults.standard.set(watchFolders, forKey: "watchFolders")
        watchSetup()
    }

    /// 暂停恢复(手动或每日上限的自动恢复)。清掉本会话跳过名单——暂停的
    /// 起因(配额/上限)往往也是这些文件失败的起因,该一起重试。
    func watchResume() {
        watchPausedReason = nil
        watchSkip.removeAll()
        watchLastIssue = nil
        watchScanSoon(after: 0)
    }

    /// 「立即扫描」:清掉跳过名单再扫,给瞬时失败(网络抖动)一条不用重启
    /// 的重试路。
    func watchScanFresh() {
        watchSkip.removeAll()
        watchLastIssue = nil
        watchScanSoon(after: 0)
    }

    /// 去抖:FSEvents 在一次导出/截图落盘时会连环触发,合并成一次扫描。
    func watchScanSoon(after seconds: Double = 2.5) {
        guard watchEnabled, connected, watchPausedReason == nil else { return }
        watchRescan?.cancel()
        watchRescan = Task { [weak self] in
            try? await Task.sleep(for: .seconds(seconds))
            guard !Task.isCancelled else { return }
            await self?.watchScan()
        }
    }

    func watchScan() async {
        guard watchEnabled, connected, watchPausedReason == nil else { return }
        // 撞上进行中的扫描不能静默丢弃:长扫描期间新落盘的文件不在本轮
        // 快照里,丢了这次请求它就只能等下一次无关变动。记账,收尾补扫。
        if watchBusy {
            watchScanAgain = true
            return
        }
        watchBusy = true
        defer { watchBusy = false }

        // 清单按 服务器+账号 键控加载;换了账号 URL 变化,自动换清单。
        let murl = watchManifestURL
        if watchManifestLoadedFrom != murl {
            let data = await Task.detached { try? Data(contentsOf: murl) }.value
            watchManifest = WatchManifest.decode(data ?? Data())
            watchManifestLoadedFrom = murl
        }

        watchStatus = "扫描中…"
        let folders = watchFolders
        // 遍历与 stat 放后台;结果是纯值,拿回主线程逐个处理。
        let found: [(path: String, size: Int64, mtime: Double)] = await Task.detached {
            var out: [(String, Int64, Double)] = []
            let fm = FileManager.default
            let keys: Set<URLResourceKey> = [.isRegularFileKey, .fileSizeKey, .contentModificationDateKey]
            for folder in folders {
                guard let en = fm.enumerator(at: URL(fileURLWithPath: folder),
                                             includingPropertiesForKeys: Array(keys),
                                             options: [.skipsHiddenFiles]) else { continue }
                while let u = en.nextObject() as? URL {
                    guard AppModel.watchExts.contains(u.pathExtension.lowercased()),
                          let rv = try? u.resourceValues(forKeys: keys),
                          rv.isRegularFile == true else { continue }
                    out.append((u.path,
                                Int64(rv.fileSize ?? 0),
                                rv.contentModificationDate?.timeIntervalSince1970 ?? 0))
                }
            }
            return out
        }.value

        var uploaded = 0, adopted = 0, failed = 0
        var pendingStability = false
        let startedAt = Date().timeIntervalSince1970

        // 按 mtime 升序:老文件先传,和用户"补历史再跟新增"的直觉一致。
        for f in found.sorted(by: { $0.mtime < $1.mtime }) {
            guard watchEnabled, watchPausedReason == nil else { break }
            if watchManifest.isCurrent(path: f.path, size: f.size, mtime: f.mtime) { continue }
            if watchSkip.contains(f.path) { continue }

            // 稳定性:还在写入的先放过,稍后重扫。有界——0 字节文件超过
            // 60 秒没动静按残留跳过;未来 mtime(时钟偏差)按已稳定处理,
            // 否则这两类文件会把监控拖进每 3 秒一轮的无限重扫。
            let age = startedAt - f.mtime
            if f.size == 0 {
                if age >= 0 && age < 60 {
                    pendingStability = true
                } else {
                    watchSkip.insert(f.path)
                    failed += 1
                }
                continue
            }
            if age >= 0 && age < 2 {
                pendingStability = true
                continue
            }

            let url = URL(fileURLWithPath: f.path)
            // 组规则只看扩展名和大小,放在算 sha 之前——被永久拒绝的大文件
            // 不该每次启动都被全量读一遍。
            if let reason = rejectLocally(url) {
                watchSkip.insert(f.path)
                failed += 1
                if watchLastIssue == nil { watchLastIssue = "部分文件被跳过:\(reason)" }
                continue
            }

            // 后台算哈希,并顺手重新 stat:串行上传会把一轮拖得很长,轮初
            // 快照到这里可能已过时——变了就交给下一轮,别哈希半成品。
            let path = f.path
            let probeTask = Task.detached { () -> (sha: String, size: Int64, mtime: Double)? in
                guard let sha = Self.sha256(of: path) else { return nil }
                let keys: Set<URLResourceKey> = [.fileSizeKey, .contentModificationDateKey]
                guard let rv = try? URL(fileURLWithPath: path).resourceValues(forKeys: keys) else { return nil }
                return (sha, Int64(rv.fileSize ?? 0),
                        rv.contentModificationDate?.timeIntervalSince1970 ?? 0)
            }
            guard let probe = await probeTask.value else {
                watchSkip.insert(path)
                failed += 1
                continue
            }
            if probe.size != f.size || abs(probe.mtime - f.mtime) > 0.001 {
                pendingStability = true
                continue
            }

            // 同内容已传过(改名/移动/复制):收编,不再占一次配额。
            if let known = watchManifest.known(sha: probe.sha) {
                watchManifest.adopt(path: path, size: f.size, mtime: f.mtime, from: known)
                adopted += 1
                continue
            }

            watchStatus = "上传 \(url.lastPathComponent)…"
            var toSend = url
            var temp: URL?
            if uploadMode == .optimized, maxImageWidth > 0,
               let smaller = LocalResize.shrink(url, maxWidth: maxImageWidth) {
                toSend = smaller
                temp = smaller
            }
            defer { if let temp { try? FileManager.default.removeItem(at: temp) } }
            // 自动水印:先缩放后打水印(水印字号相对最终尺寸才对)。原图
            // 模式不加——字节原样是那个模式的承诺;动图渲染返回 nil,自然
            // 落回原件。
            var editTemp: URL?
            if wmAutoWatch, uploadMode == .optimized, let wm = watermarkSpec() {
                var wmSpec = EditSpec()
                wmSpec.watermark = wm
                let wmSource = toSend
                let wmTask = Task.detached { ImageEdit.render(source: wmSource, spec: wmSpec) }
                if let out = await wmTask.value {
                    toSend = out
                    editTemp = out
                }
            }
            defer { if let editTemp { try? FileManager.default.removeItem(at: editTemp.deletingLastPathComponent()) } }

            do {
                let res = try await watchUploadWithBackoff(toSend, filename: url.lastPathComponent)
                watchManifest.record(.init(path: path, size: f.size, mtime: f.mtime,
                                           sha256: probe.sha, imageID: res.image.id, url: res.image.url))
                uploaded += 1
                // 初次收录整目录可能跑很久,每 25 张存一次清单——⌘Q 或崩溃
                // 最多丢 25 条记录,而不是一整轮。
                if uploaded % 25 == 0 { await watchSaveManifest() }
            } catch {
                if watchShouldPause(error) {
                    watchPausedReason = message(error)
                    if case OpenimgError.dailyLimitReached = error {
                        watchScheduleAutoResume()
                    }
                    break
                }
                watchSkip.insert(path)
                failed += 1
            }
        }

        // 清单落盘必须无条件——已上传的记录丢了就是重复扣配额。
        await watchSaveManifest()

        // 以下只是界面反馈与续接调度:扫描中被禁用或登出后不该再冒出来。
        guard watchEnabled, connected else { return }
        if uploaded > 0 {
            announce("目录监控:新上传 \(uploaded) 张")
            quota = try? await client().quota()
            // 后台上传不打断前台浏览:深翻页或有选择时不动列表,状态行的
            // 「本次 +N」足够提示。
            if (section != .gallery || page == 0), selection.isEmpty {
                await load(resetPage: true)
            }
        }
        var line = "清单 \(watchManifest.count) 条"
        if uploaded > 0 { line += " · 本次 +\(uploaded)" }
        if adopted > 0 { line += " · 收编 \(adopted)" }
        if failed > 0 { line += " · 跳过 \(failed)" }
        watchStatus = line
        if watchScanAgain {
            watchScanAgain = false
            watchScanSoon(after: 0.5)
        } else if pendingStability {
            watchScanSoon(after: 3)
        }
    }

    private func watchSaveManifest() async {
        let snapshot = watchManifest
        let murl = watchManifestURL
        await Task.detached {
            guard let data = try? snapshot.encoded() else { return }
            try? FileManager.default.createDirectory(at: murl.deletingLastPathComponent(),
                                                     withIntermediateDirectories: true)
            try? data.write(to: murl, options: .atomic)
        }.value
    }

    /// 每日上限的暂停在次日 UTC 零点自动恢复(服务器按 UTC 记当日条数),
    /// +60 秒时钟余量。复用 watchRescan 槽:watchStop/signOut 顺带清理。
    private func watchScheduleAutoResume() {
        var cal = Calendar(identifier: .gregorian)
        cal.timeZone = TimeZone(identifier: "UTC") ?? .current
        guard let next = cal.nextDate(after: Date(),
                                      matching: DateComponents(hour: 0, minute: 0, second: 0),
                                      matchingPolicy: .nextTime) else { return }
        let delay = next.timeIntervalSinceNow + 60
        watchRescan?.cancel()
        watchRescan = Task { [weak self] in
            try? await Task.sleep(for: .seconds(delay))
            guard !Task.isCancelled else { return }
            self?.watchResume()
        }
    }

    /// 限流退避:整目录初次收录很容易撞每分钟上限,撞了等它说的秒数再试
    /// 一次;还撞才交给暂停逻辑。别的错误不重试——同一个文件失败两次的
    /// 原因不会因为立刻再试一次而消失。
    private func watchUploadWithBackoff(_ file: URL, filename: String) async throws -> UploadResponse {
        do {
            return try await client().upload(fileURL: file, filename: filename)
        } catch let OpenimgError.rateLimited(retryAfter) {
            watchStatus = "限流,\(retryAfter) 秒后继续…"
            try? await Task.sleep(for: .seconds(Double(max(1, retryAfter)) + 0.5))
            return try await client().upload(fileURL: file, filename: filename)
        }
    }

    /// 这些错误注定压垮整批,暂停等用户处理(每日上限会自动恢复);其余
    /// (单文件被拒、网络抖动)跳过继续,「立即扫描」可清跳过名单重试。
    private func watchShouldPause(_ error: Error) -> Bool {
        switch error as? OpenimgError {
        case .quotaExhausted, .dailyLimitReached, .unauthorized, .rateLimited:
            true
        default:
            false
        }
    }

    /// 流式 SHA-256。读错误显式返回 nil——用 try? 会把 I/O 错误当成 EOF,
    /// 返回半个文件的"合法"哈希。
    nonisolated private static func sha256(of path: String) -> String? {
        guard let fh = FileHandle(forReadingAtPath: path) else { return nil }
        defer { try? fh.close() }
        var hasher = SHA256()
        while true {
            let chunk: Data?
            do { chunk = try fh.read(upToCount: 1 << 20) } catch { return nil }
            guard let chunk, !chunk.isEmpty else { break }
            hasher.update(data: chunk)
        }
        return hasher.finalize().map { String(format: "%02x", $0) }.joined()
    }
}
