import Foundation
import OpenimgKit

/// 图库导出进度。汇总走吐司;finished 后控件退回按钮。
struct ExportProgress: Equatable {
    var total = 0
    var done = 0
    var skipped = 0
    var failed = 0
    var finished = false
    /// 中途分页出错(断网/令牌失效)时的原因——有它就不能汇报"导出完成"。
    var errorMessage: String?

    var processed: Int { done + skipped + failed }
}

/// 一次性导出:把整个图库(存储的主图)下载到本地目录。
///
/// 刻意不是持续同步——只增不删、可中断续传:文件名取对象键 basename
/// (与外链一致),已存在即跳过,所以中断后重跑只补缺的。
/// 注意导出的是**存储产物**:开着自动转换时是 .webp/.avif,不是上传前
/// 的原文件;要字节原样得用原图模式。
extension AppModel {
    func exportAll(to dir: URL) async {
        guard connected, export == nil || export?.finished == true else { return }
        exportCancelled = false
        export = ExportProgress()
        let limit = 200
        var offset = 0
        var seenTotal = -1
        // 秒传克隆的行共享同一个对象键,basename 相同、内容相同——去重,
        // 也顺带避免同名并发下载互抢 .part 文件。
        var seenNames = Set<String>()
        do {
            while !exportCancelled {
                // oldest 固定排序:导出期间新上传只会追加在还没翻到的尾部。
                let page = try await client().images(limit: limit, offset: offset, sort: .oldest)
                // 导出期间有删除:已翻区间左移,offset 窗口会跳过等量的行。
                // total 缩水就归零重走——「已存在即跳过」让重走幂等且便宜。
                if seenTotal >= 0, page.total < seenTotal {
                    offset = 0
                    seenTotal = page.total
                    seenNames.removeAll()
                    export = ExportProgress(total: page.total)
                    continue
                }
                seenTotal = page.total
                if export?.total == 0 { export?.total = page.total }
                if page.images.isEmpty { break }

                var fresh: [RemoteImage] = []
                for img in page.images {
                    if seenNames.insert(Self.exportName(img)).inserted {
                        fresh.append(img)
                    } else {
                        export?.skipped += 1
                    }
                }

                // 并发 4、按批推进:任务组闭包不在主 actor 上,里面只碰
                // Sendable 值,结果整批拿回主线程记账;取消粒度一批,够灵敏。
                var idx = 0
                while idx < fresh.count, !exportCancelled {
                    let chunk = Array(fresh[idx..<min(idx + 4, fresh.count)])
                    idx += chunk.count
                    let results = await withTaskGroup(of: (ok: Bool, skipped: Bool).self) { group in
                        for img in chunk {
                            group.addTask { await Self.download(img, to: dir) }
                        }
                        var out: [(ok: Bool, skipped: Bool)] = []
                        for await r in group { out.append(r) }
                        return out
                    }
                    for r in results {
                        if r.skipped {
                            export?.skipped += 1
                        } else if r.ok {
                            export?.done += 1
                        } else {
                            export?.failed += 1
                        }
                    }
                }
                offset += page.images.count
                if offset >= page.total { break }
            }
        } catch {
            export?.errorMessage = message(error)
        }
        export?.finished = true
        if let e = export {
            let counts = "新下载 \(e.done),已存在跳过 \(e.skipped)" + (e.failed > 0 ? ",失败 \(e.failed)" : "")
            let line = if let err = e.errorMessage {
                "导出中断:\(err)(\(counts))"
            } else if exportCancelled {
                "导出已取消:\(counts)"
            } else {
                "导出完成:\(counts)"
            }
            announce(line, seconds: 8)
        }
    }

    func exportCancel() { exportCancelled = true }

    func exportDismiss() { export = nil }

    nonisolated static func exportName(_ img: RemoteImage) -> String {
        if let u = URL(string: img.url), !u.lastPathComponent.isEmpty, u.lastPathComponent != "/" {
            return u.lastPathComponent
        }
        return "\(img.id).\(img.ext)"
    }

    /// 下载一张到目标目录。对象 URL 是公开 CDN 地址,刻意不带令牌
    /// (与 Client.fetchData 同一条纪律:令牌不发给第三方存储主机)。
    nonisolated private static func download(_ img: RemoteImage, to dir: URL) async -> (Bool, Bool) {
        guard let u = URL(string: img.url) else { return (false, false) }
        let dest = dir.appendingPathComponent(exportName(img))
        let fm = FileManager.default
        if fm.fileExists(atPath: dest.path) { return (true, true) }
        do {
            let (data, resp) = try await URLSession.shared.data(from: u)
            guard let http = resp as? HTTPURLResponse,
                  (200..<300).contains(http.statusCode), !data.isEmpty else {
                return (false, false)
            }
            // 先落 .part 再改名:目录里永远不会出现半个文件,存在即完整,
            // 这正是"已存在即跳过"能当续传判据的前提。
            let part = dest.appendingPathExtension("part")
            try data.write(to: part, options: .atomic)
            try? fm.removeItem(at: dest)
            try fm.moveItem(at: part, to: dest)
            return (true, false)
        } catch {
            return (false, false)
        }
    }
}
