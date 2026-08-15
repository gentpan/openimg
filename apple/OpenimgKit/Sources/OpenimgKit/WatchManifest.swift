import Foundation

/// 监控目录的本地上传清单。
///
/// 存在的理由:服务端秒传命中虽然不重存对象,但会**新建一行、新短链、照扣
/// 配额**(防白嫖是刻意设计)。监控器重启后要重扫目录,不记得哪些传过就会
/// 每次都新建记录、重复吃掉用户空间——清单是这个功能里最重要的一块,不是
/// 优化。
///
/// 纯数据逻辑,不碰文件系统(读写由调用方传 Data 进出),所以 KitCheck 能在
/// 没有磁盘状态的环境里自检它。
public struct WatchManifest: Sendable {
    public struct Entry: Codable, Sendable, Equatable {
        public var path: String
        public var size: Int64
        /// 秒级 mtime。比对用它 + size 做快路径:两者都没变就不必重新算
        /// sha——这让每次全量扫描对已收录文件几乎零成本。
        public var mtime: Double
        public var sha256: String
        public var imageID: String
        public var url: String

        public init(path: String, size: Int64, mtime: Double,
                    sha256: String, imageID: String, url: String) {
            self.path = path
            self.size = size
            self.mtime = mtime
            self.sha256 = sha256
            self.imageID = imageID
            self.url = url
        }
    }

    /// path → Entry。sha 索引是派生的,不入编码。
    private var byPath: [String: Entry] = [:]
    private var bySha: [String: Entry] = [:]

    public init() {}

    public var count: Int { byPath.count }

    /// 快路径:路径已收录且 size/mtime 未变。变了不代表要上传——内容可能
    /// 没变(触摸过 mtime),调用方应重新算 sha 再问 `known(sha:)`。
    public func isCurrent(path: String, size: Int64, mtime: Double) -> Bool {
        guard let e = byPath[path] else { return false }
        return e.size == size && Int(e.mtime) == Int(mtime)
    }

    /// 内容已上传过(可能在别的路径——重命名/移动/复制)。
    public func known(sha: String) -> Entry? { bySha[sha] }

    public func entry(path: String) -> Entry? { byPath[path] }

    /// 收录一次成功上传。
    public mutating func record(_ e: Entry) {
        byPath[e.path] = e
        bySha[e.sha256] = e
    }

    /// 收编改名/移动/复制:同内容换了路径,不重新上传,把新路径指到已有的
    /// 上传记录上。旧路径条目保留——文件被复制时两个路径都合法。
    public mutating func adopt(path: String, size: Int64, mtime: Double, from known: Entry) {
        byPath[path] = Entry(path: path, size: size, mtime: mtime,
                             sha256: known.sha256, imageID: known.imageID, url: known.url)
    }

    // MARK: - 序列化

    private struct Envelope: Codable {
        var version: Int
        var entries: [Entry]
    }

    public func encoded() throws -> Data {
        let env = Envelope(version: 1, entries: byPath.values.sorted { $0.path < $1.path })
        let enc = JSONEncoder()
        enc.outputFormatting = [.prettyPrinted, .sortedKeys]
        return try enc.encode(env)
    }

    public static func decode(_ data: Data) -> WatchManifest {
        var m = WatchManifest()
        // 清单损坏就当空的重来:代价只是重扫时多算一遍 sha,秒传命中的行
        // 会重复建(一次性),比让整个监控功能崩掉好。
        guard let env = try? JSONDecoder().decode(Envelope.self, from: data) else { return m }
        for e in env.entries { m.record(e) }
        return m
    }
}
