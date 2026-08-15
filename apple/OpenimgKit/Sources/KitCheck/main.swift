import Foundation
import ImageIO
import UniformTypeIdentifiers
@testable import OpenimgKit

// Self-checks for OpenimgKit. Run with `swift run KitCheck`; exits non-zero on
// the first failure so CI can gate on it. See Package.swift for why this is an
// executable and not a test target.

var failures = 0
var checks = 0

// @MainActor because top-level code in main.swift is main-actor isolated under
// Swift 6, and these mutate counters that live there.
@MainActor
func check(_ name: String, _ ok: @autoclosure () -> Bool) {
    checks += 1
    if ok() {
        print("  ✓ \(name)")
    } else {
        print("  ✗ \(name)")
        failures += 1
    }
}

@MainActor
func section(_ s: String) { print("\n\(s)") }

// MARK: - Server address

section("服务器地址")

// The token rides in a header on every request, so a typo'd hostname over
// plain http hands it to whoever answers.
do {
    _ = try OpenimgClient(server: URL(string: "http://example.com")!, token: "oimg_x")
    check("明文 http 被拒绝", false)
} catch let e as OpenimgError {
    check("明文 http 被拒绝", e == .insecureServer(host: "example.com"))
} catch {
    check("明文 http 被拒绝", false)
}

for host in ["http://localhost:8080", "http://127.0.0.1:8080", "https://openimg.io"] {
    var ok = false
    do {
        _ = try OpenimgClient(server: URL(string: host)!, token: "oimg_x")
        ok = true
    } catch { ok = false }
    check("\(host) 放行", ok)
}

// MARK: - Error mapping

section("错误映射")

let resp429 = HTTPURLResponse(url: URL(string: "https://x")!, statusCode: 429,
                              httpVersion: nil, headerFields: nil)!

// 429 arrives for two unrelated reasons. The per-minute limit clears on its
// own; the daily upload count does not clear until tomorrow. A tool that
// uploads screenshots hits the second one far more often, and telling the user
// to "try again shortly" there is simply wrong.
check("每日上限识别为 dailyLimitReached",
      OpenimgClient.failure(status: 429,
                            body: Data(#"{"error":"今日上传数量已达上限","used":50,"limit":50}"#.utf8),
                            headers: resp429) == .dailyLimitReached(used: 50, limit: 50))

check("突发限流识别为 rateLimited",
      OpenimgClient.failure(status: 429,
                            body: Data(#"{"error":"上传过于频繁","retry_after":37}"#.utf8),
                            headers: resp429) == .rateLimited(retryAfter: 37))

let respRetry = HTTPURLResponse(url: URL(string: "https://x")!, statusCode: 429,
                                httpVersion: nil, headerFields: ["Retry-After": "12"])!
check("响应体没带秒数时回退到 Retry-After 头",
      OpenimgClient.failure(status: 429, body: Data("{}".utf8), headers: respRetry)
        == .rateLimited(retryAfter: 12))

let resp507 = HTTPURLResponse(url: URL(string: "https://x")!, statusCode: 507,
                              httpVersion: nil, headerFields: nil)!
check("507 识别为配额耗尽",
      OpenimgClient.failure(status: 507, body: Data(#"{"error":"空间不足"}"#.utf8),
                            headers: resp507) == .quotaExhausted("空间不足"))

let resp401 = HTTPURLResponse(url: URL(string: "https://x")!, statusCode: 401,
                              httpVersion: nil, headerFields: nil)!
check("401 识别为令牌无效",
      OpenimgClient.failure(status: 401, body: Data(#"{"error":"auth: token expired"}"#.utf8),
                            headers: resp401) == .unauthorized("auth: token expired"))

// MARK: - Multipart

section("multipart 组装")

// A filename carrying a quote or a newline would otherwise break out of the
// Content-Disposition header and let the caller forge extra parts.
let hdr = MultipartBody.header(fieldName: "file", filename: "a\"b\r\nc.png", boundary: "B")
check("文件名里的引号被转义", hdr.contains(#"filename="a\"bc.png""#))
check("文件名里的换行被去掉", !hdr.contains("\na"))
// The blank line is what separates headers from body. Without it the server
// reads the image bytes as more header lines and the part is malformed.
check("头部以空行收尾", hdr.hasSuffix("\r\n\r\n"))
check("头部恰好三行", hdr.components(separatedBy: "\r\n").count == 5)

check("HEIC 的 MIME", MultipartBody.mimeType(for: "photo.HEIC") == "image/heic")
check("未知扩展名回退", MultipartBody.mimeType(for: "x.zzz") == "application/octet-stream")

do {
    let dir = FileManager.default.temporaryDirectory
    let src = dir.appendingPathComponent("kitcheck-\(UUID().uuidString).png")
    let payload = Data((0..<300_000).map { UInt8($0 % 251) }) // 越过 1 MB 分块边界
    try payload.write(to: src)
    defer { try? FileManager.default.removeItem(at: src) }

    let out = try MultipartBody.write(fileURL: src, fieldName: "file",
                                      filename: "t.png", boundary: "BOUND")
    defer { try? FileManager.default.removeItem(at: out) }
    let body = try Data(contentsOf: out)

    check("以 boundary 开头", body.starts(with: Data("--BOUND\r\n".utf8)))
    check("以结束 boundary 收尾", body.suffix(13) == Data("\r\n--BOUND--\r\n".utf8))
    check("图片内容在分块写入中未被改动", body.range(of: payload) != nil)
} catch {
    check("multipart 写入不抛错（\(error)）", false)
}

// MARK: - Decoding

section("响应解析")

let dec = JSONDecoder()
dec.dateDecodingStrategy = .iso8601WithFractionalSeconds

// Go's time.Time marshals with nanoseconds, and .iso8601 rejects that outright
// — one fractional second makes the entire response fail to decode.
let withNanos = """
{"id":"1","orig_name":"a.png","ext":"png","width":10,"height":8,"size_stored":123,
 "url":"https://cdn/x.png","thumb_url":"https://cache/x.webp","markdown":"M",
 "html":"H","bbcode":"B","created_at":"2026-08-05T14:32:08.123456789Z"}
"""
if let img = try? dec.decode(RemoteImage.self, from: Data(withNanos.utf8)) {
    check("带纳秒的时间可解析", true)
    check("short_url 缺失不影响解析", img.shortURL == nil)
    check("链接格式取服务端拼好的字符串", LinkFormat.markdown.render(img) == "M")
    check("四种链接格式齐全", LinkFormat.allCases.count == 4)
} else {
    check("带纳秒的时间可解析", false)
}

let plain = """
{"id":"1","orig_name":"a.png","ext":"png","width":1,"height":1,"size_stored":1,
 "url":"U","thumb_url":"T","markdown":"M","html":"H","bbcode":"B",
 "created_at":"2026-08-05T14:32:08Z"}
"""
check("不带小数秒的时间同样可解析",
      (try? dec.decode(RemoteImage.self, from: Data(plain.utf8))) != nil)

// The tier block is what lets a client reject a file locally instead of
// spending one of the day's uploads finding the limit out from a 413.
let quotaJSON = """
{"quota_bytes":1073741824,"used_bytes":1024,"available_bytes":1073740800,
 "image_count":3,"uploads_today":7,
 "tier":{"name":"free","max_file_size":20971520,"daily_upload_count":50,
         "allowed_formats":["jpeg","png","webp"]}}
"""
if let q = try? dec.decode(Quota.self, from: Data(quotaJSON.utf8)) {
    check("配额解析", q.availableBytes == 1_073_740_800)
    check("档位限制解析", q.tier.maxFileSize == 20_971_520 && q.tier.dailyUploadCount == 50)
    check("允许格式解析", q.tier.allowedFormats.contains("png"))
} else {
    check("配额解析", false)
}

// 服务端的 avatar_url 带 omitempty：没有头像的账号响应里根本没有这个键，
// 声明成非可选会让整条账号解析失败，而症状是"登录不上"而不是"头像没了"。
section("账号解析")

let withAvatar = #"{"id":"1","email":"a@b.c","name":"西风","role":"admin","avatar_url":"https://cdn/x.avif"}"#
if let a = try? dec.decode(Account.self, from: Data(withAvatar.utf8)) {
    check("带头像可解析", a.avatarURL == "https://cdn/x.avif")
    check("首字母取昵称而非邮箱", a.initial == "西")
} else {
    check("带头像可解析", false)
}

let noAvatar = #"{"id":"1","email":"zoe@b.c","name":"","role":"user"}"#
if let a = try? dec.decode(Account.self, from: Data(noAvatar.utf8)) {
    check("缺 avatar_url 仍可解析", a.avatarURL == nil)
    check("无昵称时首字母回退到邮箱", a.initial == "Z")
} else {
    check("缺 avatar_url 仍可解析", false)
}

section("Passkey 解析")

// 服务端用 time.RFC3339 格式化，不带小数秒；而客户端解码器全局设的是
// iso8601WithFractionalSeconds。这两处对不上会在运行时整个响应解析失败。
let pkJSON = """
{"passkeys":[
 {"id":"a1","name":"MacBook Pro","created_at":"2026-08-01T10:30:00Z",
  "last_used_at":"2026-08-05T18:00:00Z"},
 {"id":"a2","name":"iPhone","created_at":"2026-07-20T09:00:00Z"}]}
"""
struct PKWrap: Decodable { let passkeys: [PasskeyCredential]? }
let pkDec = JSONDecoder()
pkDec.dateDecodingStrategy = .iso8601WithFractionalSeconds   // 与 Client 同款
let pk = try? pkDec.decode(PKWrap.self, from: Data(pkJSON.utf8))
check("RFC3339 无小数秒也能解析", pk?.passkeys?.count == 2)
check("解出了创建时间", pk?.passkeys?.first?.createdAt != nil)
check("缺 last_used_at 不影响解析", pk?.passkeys?.last?.lastUsedAt == nil)
check("外层 key 是 passkeys 而非 credentials", pk?.passkeys?.first?.name == "MacBook Pro")
check("时间戳无法解析时返回 nil 而不是抛错",
      PasskeyCredential.date("不是时间") == nil && PasskeyCredential.date(nil) == nil)

section("图库网格")

// 默认窗口 1240x820 下网格的可用区。这两个数是从离屏渲染里打印出来的实测
// 值，不是从内边距推算的——推算过一次，结果差了 180pt。
let deskGrid = CGSize(width: 972, height: 637)
let f50 = GridFit.solve(count: 50, in: deskGrid, spacing: 12, minCell: 72)
check("50 张不用滚动", !f50.scrolls)
check("行列容得下 50 张（\(f50.columns)x\(f50.rows)）", f50.columns * f50.rows >= 50)
check("横向不溢出",
      Double(f50.columns) * f50.cellWidth + 12 * Double(f50.columns - 1) <= 972.5)
check("纵向不溢出",
      Double(f50.rows) * f50.cellHeight + 12 * Double(f50.rows - 1) <= 637.5)
check("格子是正方（\(Int(f50.cellWidth))x\(Int(f50.cellHeight))）",
      f50.cellWidth == f50.cellHeight)
check("格子不至于太小", f50.cellWidth >= 72)

// 换页大小：都要能铺满，且张数越多格子越小
let sizes = [25, 50, 100, 200].map { GridFit.solve(count: $0, in: deskGrid, spacing: 12, minCell: 72) }
// 默认窗口在 72pt 下限内最多摆 13x7=91 格，所以 100/200 注定要滚动。
// 这正是想要的：宁可滚动，也不把格子压到看不清。
check("25/50 铺满不滚动", !sizes[0].scrolls && !sizes[1].scrolls)
check("100/200 改为滚动而不是压小格子",
      sizes[2].scrolls && sizes[3].scrolls
      && sizes[2].cellWidth >= 72 && sizes[3].cellWidth >= 72)
check("张数越多格子越小",
      sizes[0].cellWidth > sizes[1].cellWidth && sizes[1].cellWidth > sizes[2].cellWidth)

// 最小窗口：宁可滚动也不要 40pt 的格子
let tiny = GridFit.solve(count: 50, in: CGSize(width: 632, height: 362), spacing: 12, minCell: 72)
check("窗口太小时滚动", tiny.scrolls)
check("滚动时格子仍不小于下限", tiny.cellWidth >= 72)

// 少量图片时不该被拉成长条
let three = GridFit.solve(count: 3, in: deskGrid, spacing: 12, minCell: 72)
check("3 张时也是正方", three.cellWidth == three.cellHeight)

// 退化输入
check("0 张不崩", GridFit.solve(count: 0, in: deskGrid, spacing: 12, minCell: 72).columns >= 1)
check("尺寸为 0 不崩", GridFit.solve(count: 50, in: .zero, spacing: 12, minCell: 72).scrolls)
check("尺寸为 NaN 不崩",
      GridFit.solve(count: 50, in: CGSize(width: CGFloat.nan, height: CGFloat.nan),
                    spacing: 12, minCell: 72).scrolls)

section("本地缩放")

/// The sample is drawn here rather than read from disk.
///
/// It used to point at a file in a scratch directory, guarded by a
/// `fileExists` check that printed "跳过" and moved on. The directory was
/// eventually cleaned, and this section quietly shrank from six assertions to
/// one — the suite still said "全部通过" while testing almost nothing. A test
/// that can skip itself is a test that will.
func makeSamplePNG(width: Int, height: Int) -> URL {
    let url = FileManager.default.temporaryDirectory
        .appendingPathComponent("kitcheck-\(width)x\(height).png")

    // Per-pixel noise, written straight into the buffer.
    //
    // The first attempt filled 4x4 blocks of flat colour, which PNG squeezed to
    // 94 KB for a 2752x1536 image — and downscaling it produced a *larger* file
    // (518 KB), because interpolation turns crisp block edges into gradients
    // that no longer run-length encode. LocalResize correctly refused to call
    // that a shrink, so the assertion failed on a fixture that was nothing like
    // a photograph. Real photographs are incompressible at the pixel level,
    // which is the property being tested.
    let bytesPerRow = width * 4
    var pixels = [UInt8](repeating: 0, count: bytesPerRow * height)
    var seed: UInt64 = 0x2545F4914F6CDD1D
    for i in stride(from: 0, to: pixels.count, by: 4) {
        seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17
        pixels[i]     = UInt8(truncatingIfNeeded: seed)
        pixels[i + 1] = UInt8(truncatingIfNeeded: seed >> 8)
        pixels[i + 2] = UInt8(truncatingIfNeeded: seed >> 16)
        pixels[i + 3] = 255
    }

    let img: CGImage = pixels.withUnsafeMutableBytes { buf in
        let ctx = CGContext(data: buf.baseAddress, width: width, height: height,
                            bitsPerComponent: 8, bytesPerRow: bytesPerRow,
                            space: CGColorSpaceCreateDeviceRGB(),
                            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        return ctx.makeImage()!
    }
    let dest = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(dest, img, nil)
    CGImageDestinationFinalize(dest)
    return url
}

let bigPNG = makeSamplePNG(width: 2752, height: 1536)
defer { try? FileManager.default.removeItem(at: bigPNG) }

let before = (try! bigPNG.resourceValues(forKeys: [.fileSizeKey]).fileSize)!

if let out = LocalResize.shrink(bigPNG, maxWidth: 1920) {
    defer { try? FileManager.default.removeItem(at: out) }
    let after = (try! out.resourceValues(forKeys: [.fileSizeKey]).fileSize)!
    let src = CGImageSourceCreateWithURL(out as CFURL, nil)!
    let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as! [CFString: Any]
    let w = props[kCGImagePropertyPixelWidth] as! Int

    check("缩到了指定宽度（\(w)px）", w == 1920)
    check("格式没变（仍是 PNG）", (CGImageSourceGetType(src)! as String) == UTType.png.identifier)
    check("体积确实变小（\(before/1024)KB → \(after/1024)KB）", after < before)
} else {
    check("2752px 的图应当被缩", false)
}

// 已经够小的不该动，白花一次编码
check("宽度已达标时返回 nil", LocalResize.shrink(bigPNG, maxWidth: 4000) == nil)
check("未设上限时返回 nil", LocalResize.shrink(bigPNG, maxWidth: 0) == nil)

check("非图片文件安全返回 nil",
      LocalResize.shrink(URL(fileURLWithPath: "/etc/hosts"), maxWidth: 1920) == nil)

// MARK: - WatchManifest

section("WatchManifest(监控目录清单)")

var wm = WatchManifest()
let we = WatchManifest.Entry(path: "/a/p.jpg", size: 100, mtime: 1000,
                             sha256: "aa11", imageID: "id1", url: "https://x/p.webp")
wm.record(we)
check("收录后快路径命中", wm.isCurrent(path: "/a/p.jpg", size: 100, mtime: 1000))
check("size 变了快路径失效", !wm.isCurrent(path: "/a/p.jpg", size: 101, mtime: 1000))
check("mtime 变了快路径失效", !wm.isCurrent(path: "/a/p.jpg", size: 100, mtime: 2000))
check("未收录路径不命中", !wm.isCurrent(path: "/a/q.jpg", size: 100, mtime: 1000))
check("按 sha 找到已传内容", wm.known(sha: "aa11")?.imageID == "id1")
check("陌生 sha 返回 nil", wm.known(sha: "bb22") == nil)

// 改名收编:同内容换路径不重传,新路径指向旧上传记录
wm.adopt(path: "/a/renamed.jpg", size: 100, mtime: 3000, from: we)
check("收编后新路径快路径命中", wm.isCurrent(path: "/a/renamed.jpg", size: 100, mtime: 3000))
check("收编保留原上传信息", wm.entry(path: "/a/renamed.jpg")?.url == "https://x/p.webp")
check("原路径仍在(复制场景两路径都合法)", wm.entry(path: "/a/p.jpg") != nil)

// 序列化往返
if let data = try? wm.encoded() {
    let back = WatchManifest.decode(data)
    check("编码往返条目数一致", back.count == wm.count)
    check("往返后 sha 索引重建", back.known(sha: "aa11")?.url == "https://x/p.webp")
} else {
    check("清单可编码", false)
}
check("损坏数据解码为空清单而非崩溃", WatchManifest.decode(Data("not json".utf8)).count == 0)

// MARK: - EditGeometry(编辑几何)

section("EditGeometry(编辑几何)")

let full = CGRect(x: 0, y: 0, width: 1, height: 1)
check("整幅裁剪夹取后不变", EditGeometry.clampCrop(full) == full)
check("越界裁剪被夹回 0-1", {
    let r = EditGeometry.clampCrop(CGRect(x: -0.2, y: 0.5, width: 0.4, height: 0.9))
    return r.minX >= 0 && r.minY >= 0 && r.maxX <= 1.0001 && r.maxY <= 1.0001
}())
check("零面积裁剪被撑到最小边", {
    let r = EditGeometry.clampCrop(CGRect(x: 0.5, y: 0.5, width: 0, height: 0))
    return r.width >= 0.02 && r.height >= 0.02
}())

check("旋转 1 次尺寸互换", EditGeometry.rotatedSize(CGSize(width: 300, height: 200), quarters: 1) == CGSize(width: 200, height: 300))
check("旋转 2 次尺寸不变", EditGeometry.rotatedSize(CGSize(width: 300, height: 200), quarters: 2) == CGSize(width: 300, height: 200))

// 顺时针 90°:左上角 (0,0) 应到右上角 (1,0)
check("点旋转:左上→右上", EditGeometry.rotateQuarterCW(CGPoint(x: 0, y: 0)) == CGPoint(x: 1, y: 0))
check("点旋转:中心不动", EditGeometry.rotateQuarterCW(CGPoint(x: 0.5, y: 0.5)) == CGPoint(x: 0.5, y: 0.5))
check("点旋转四次回原位", {
    var p = CGPoint(x: 0.2, y: 0.7)
    for _ in 0..<4 { p = EditGeometry.rotateQuarterCW(p) }
    return abs(p.x - 0.2) < 1e-9 && abs(p.y - 0.7) < 1e-9
}())
check("矩形旋转四次回原位", {
    var r = CGRect(x: 0.1, y: 0.2, width: 0.3, height: 0.4)
    for _ in 0..<4 { r = EditGeometry.rotateQuarterCW(r) }
    return abs(r.minX - 0.1) < 1e-9 && abs(r.minY - 0.2) < 1e-9
        && abs(r.width - 0.3) < 1e-9 && abs(r.height - 0.4) < 1e-9
}())

check("配方整体旋转:计数+1 且笔迹跟转", {
    var s = EditSpec()
    s.strokes = [MosaicStroke(points: [CGPoint(x: 0, y: 0)], radius: 0.02)]
    let r = EditGeometry.rotateSpecCW(s)
    return r.rotationQuarters == 1 && r.strokes[0].points[0] == CGPoint(x: 1, y: 0)
        && r.strokes[0].radius == 0.02
}())

// 九宫格(CG 坐标 y 向上):锚 0=视觉左上 → 高 y;锚 8=视觉右下 → 低 y
let ts = CGSize(width: 100, height: 20)
let cv = CGSize(width: 1000, height: 500)
check("水印锚点:左上", {
    let o = EditGeometry.watermarkOrigin(anchor: 0, textSize: ts, canvas: cv, margin: 10)
    return o.x == 10 && o.y == 500 - 20 - 10
}())
check("水印锚点:居中", {
    let o = EditGeometry.watermarkOrigin(anchor: 4, textSize: ts, canvas: cv, margin: 10)
    return o.x == 450 && o.y == 240
}())
check("水印锚点:右下", {
    let o = EditGeometry.watermarkOrigin(anchor: 8, textSize: ts, canvas: cv, margin: 10)
    return o.x == 1000 - 100 - 10 && o.y == 10
}())

check("比例约束 1:1 得到像素正方形", {
    let r = EditGeometry.applyRatio(CGRect(x: 0.1, y: 0.1, width: 0.8, height: 0.8),
                                    pixelRatio: 1, canvas: CGSize(width: 2000, height: 1000))
    let pw = r.width * 2000, ph = r.height * 1000
    return abs(pw - ph) < 1
}())

// 锚点版比例约束:拖角时固定角必须钉死(中心重锚版会让它漂移)
check("锚点比例:固定角钉死且像素比恒真", {
    let cv = CGSize(width: 2000, height: 1000)
    let r = EditGeometry.applyRatio(anchor: CGPoint(x: 0.2, y: 0.2),
                                    cursor: CGPoint(x: 0.9, y: 0.4),
                                    pixelRatio: 1, canvas: cv)
    let anchorFixed = abs(r.minX - 0.2) < 1e-9 && abs(r.minY - 0.2) < 1e-9
    let pw = r.width * 2000, ph = r.height * 1000
    return anchorFixed && abs(pw - ph) < 0.5
}())
check("锚点比例:反方向拖动锚在右下", {
    let cv = CGSize(width: 1000, height: 1000)
    let r = EditGeometry.applyRatio(anchor: CGPoint(x: 0.8, y: 0.8),
                                    cursor: CGPoint(x: 0.2, y: 0.5),
                                    pixelRatio: 1, canvas: cv)
    return abs(r.maxX - 0.8) < 1e-9 && abs(r.maxY - 0.8) < 1e-9
}())
check("锚点比例:顶到画布边等比缩回不越界", {
    let cv = CGSize(width: 1000, height: 1000)
    let r = EditGeometry.applyRatio(anchor: CGPoint(x: 0.9, y: 0.9),
                                    cursor: CGPoint(x: 2.0, y: 2.0),
                                    pixelRatio: 1, canvas: cv)
    return r.maxX <= 1.0001 && r.maxY <= 1.0001 && abs(r.width - r.height) < 1e-6
}())

// 水印文字度量:CJK 每字约 1em,估算 0.62em 那种偏差必须被同源度量取代
check("水印度量:CJK 宽度接近 1em/字", {
    let s = ImageEdit.watermarkTextSize("水印文字", fontSize: 24)
    return s.width > 24 * 4 * 0.9 && s.width < 24 * 4 * 1.2
}())

// MARK: - ImageEdit(渲染冒烟)

section("ImageEdit(渲染冒烟)")

// 32x16 纯色 PNG 走完整管线:旋转+裁剪+马赛克+水印
let editPNG: URL = {
    let ctx = CGContext(data: nil, width: 32, height: 16, bitsPerComponent: 8, bytesPerRow: 0,
                        space: CGColorSpace(name: CGColorSpace.sRGB)!,
                        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    ctx.setFillColor(CGColor(red: 0.2, green: 0.5, blue: 0.9, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: 32, height: 16))
    let u = FileManager.default.temporaryDirectory.appendingPathComponent("kitcheck-edit.png")
    let d = CGImageDestinationCreateWithURL(u as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(d, ctx.makeImage()!, nil)
    CGImageDestinationFinalize(d)
    return u
}()
defer { try? FileManager.default.removeItem(at: editPNG) }

check("静态图可编辑", ImageEdit.editable(editPNG))
check("空配方不渲染(返回 nil)", ImageEdit.render(source: editPNG, spec: EditSpec()) == nil)
if true {
    var s = EditSpec()
    s.rotationQuarters = 1
    s.crop = CGRect(x: 0, y: 0, width: 0.5, height: 0.5)
    s.strokes = [MosaicStroke(points: [CGPoint(x: 0.3, y: 0.3), CGPoint(x: 0.7, y: 0.7)], radius: 0.05)]
    s.watermark = WatermarkSpec(text: "openimg.io")
    if let out = ImageEdit.render(source: editPNG, spec: s),
       let src = CGImageSourceCreateWithURL(out as CFURL, nil),
       let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
       let w = props[kCGImagePropertyPixelWidth] as? Int,
       let h = props[kCGImagePropertyPixelHeight] as? Int {
        // 32x16 转 90° 后 16x32,再裁一半 → 8x16
        check("全管线渲染:旋转+裁剪后尺寸正确", w == 8 && h == 16)
        check("产物保留源文件名", out.deletingPathExtension().lastPathComponent == "kitcheck-edit")
        try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
    } else {
        check("全管线渲染成功", false)
    }
}

// 渲染位置断言:尺寸对不等于方向对——旋转/裁剪的坐标系颠倒在尺寸上不可见。
// 4x2 图:左上角一颗红像素,其余蓝。顺时针 90° 后红点应在右上角;
// 裁掉左半后应不含红点。
func pixelAt(_ url: URL, _ x: Int, _ y: Int) -> (r: UInt8, g: UInt8, b: UInt8)? {
    guard let src = CGImageSourceCreateWithURL(url as CFURL, nil),
          let img = CGImageSourceCreateImageAtIndex(src, 0, nil),
          let ctx = CGContext(data: nil, width: img.width, height: img.height,
                              bitsPerComponent: 8, bytesPerRow: img.width * 4,
                              space: CGColorSpace(name: CGColorSpace.sRGB)!,
                              bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
    else { return nil }
    ctx.draw(img, in: CGRect(x: 0, y: 0, width: img.width, height: img.height))
    guard let data = ctx.data else { return nil }
    // CGContext 的坐标 y 向上,但底层缓冲区首行是视觉顶行——内存本来就是
    // 自顶向下的,直接用视觉 y 当行号,不要再翻一次。
    let p = data.advanced(by: (y * img.width + x) * 4).assumingMemoryBound(to: UInt8.self)
    return (p[0], p[1], p[2])
}

let posPNG: URL = {
    let ctx = CGContext(data: nil, width: 4, height: 2, bitsPerComponent: 8, bytesPerRow: 0,
                        space: CGColorSpace(name: CGColorSpace.sRGB)!,
                        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    ctx.setFillColor(CGColor(red: 0, green: 0, blue: 1, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: 4, height: 2))
    ctx.setFillColor(CGColor(red: 1, green: 0, blue: 0, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 1, width: 1, height: 1))   // CG y 向上:视觉左上角
    let u = FileManager.default.temporaryDirectory.appendingPathComponent("kitcheck-pos.png")
    let d = CGImageDestinationCreateWithURL(u as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(d, ctx.makeImage()!, nil)
    CGImageDestinationFinalize(d)
    return u
}()
defer { try? FileManager.default.removeItem(at: posPNG) }

if let p = pixelAt(posPNG, 0, 0), p.r > 200, p.b < 60 {
    check("位置基准:源图左上角是红", true)
} else {
    check("位置基准:源图左上角是红", false)
}
if true {
    var s = EditSpec()
    s.rotationQuarters = 1
    if let out = ImageEdit.render(source: posPNG, spec: s),
       let tr = pixelAt(out, 1, 0), let tl = pixelAt(out, 0, 0) {
        // 视觉顺时针 90°:左上角 → 右上角(2x4 图的右上是 x=1,y=0)
        check("旋转方向:红点到右上角", tr.r > 200 && tr.b < 60 && tl.b > 200)
        try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
    } else {
        check("旋转方向:红点到右上角", false)
    }
}
if true {
    var s = EditSpec()
    s.crop = CGRect(x: 0.5, y: 0, width: 0.5, height: 1)   // 右半
    if let out = ImageEdit.render(source: posPNG, spec: s),
       let p0 = pixelAt(out, 0, 0), let p1 = pixelAt(out, 1, 0) {
        check("裁剪位置:右半不含红点", p0.b > 200 && p1.b > 200 && p0.r < 60)
        try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
    } else {
        check("裁剪位置:右半不含红点", false)
    }
}
if true {
    var s = EditSpec()
    s.crop = CGRect(x: 0, y: 0, width: 0.25, height: 0.5)   // 左上角那格
    if let out = ImageEdit.render(source: posPNG, spec: s),
       let p0 = pixelAt(out, 0, 0) {
        check("裁剪位置:左上角保留红点", p0.r > 200 && p0.b < 60)
        try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
    } else {
        check("裁剪位置:左上角保留红点", false)
    }
}
if true {
    var s = EditSpec()
    s.strokes = [MosaicStroke(points: [CGPoint(x: 0.125, y: 0.25)], radius: 0.12)]
    s.mosaicStyle = .solid
    if let out = ImageEdit.render(source: posPNG, spec: s),
       let p0 = pixelAt(out, 0, 0), let p3 = pixelAt(out, 3, 1) {
        // 纯色涂抹在视觉左上角:红点被抹黑,远端右下角保持蓝
        check("马赛克位置:左上角被涂黑,远角不受影响",
              p0.r < 60 && p0.b < 60 && p3.b > 200)
        try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
    } else {
        check("马赛克位置:左上角被涂黑,远角不受影响", false)
    }
}

// MARK: - Result

print("\n\(checks - failures)/\(checks) 通过")
if failures > 0 {
    print("\(failures) 项失败")
    exit(1)
}
