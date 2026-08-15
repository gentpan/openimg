import Foundation
import ImageIO
import CoreGraphics
import CoreText
import UniformTypeIdentifiers

/// 上传前编辑的"配方":旋转 → 马赛克 → 裁剪 → 水印,渲染时按此顺序应用。
///
/// 所有坐标归一化到 0-1,基于**当前旋转后的画面**(用户所见即所记);再转
/// 90° 时由 EditGeometry 把已有笔迹/裁剪框一起转过去。笔刷半径归一化到
/// 对角线——旋转不改变对角线,半径含义因此旋转不变。
///
/// 这里只做服务端做不了的**内容决策**(涂掉隐私、裁掉无关、打署名)。
/// 压缩与转格式仍归服务端:本地转码会破坏 SHA 秒传、叠加两代有损——见
/// LocalResize 头注,同一条纪律。
public struct EditSpec: Sendable, Equatable {
    /// 顺时针 90° 次数,0-3。
    public var rotationQuarters = 0
    /// 归一化裁剪区,nil 不裁。
    public var crop: CGRect?
    public var strokes: [MosaicStroke] = []
    public var mosaicStyle: MosaicStyle = .pixelate
    public var watermark: WatermarkSpec?

    public init() {}

    /// 有任何会改变像素的操作。
    public var hasEdits: Bool {
        rotationQuarters % 4 != 0 || crop != nil || !strokes.isEmpty || watermark != nil
    }
}

public struct MosaicStroke: Sendable, Equatable {
    /// 归一化点列(基于旋转后画面)。
    public var points: [CGPoint]
    /// 笔刷半径,归一化到画面对角线。
    public var radius: Double

    public init(points: [CGPoint], radius: Double) {
        self.points = points
        self.radius = radius
    }
}

/// 像素化会破坏信息,纯色覆盖更彻底;都不用高斯模糊——模糊文字有被复原
/// 的先例,给用户一个"看起来遮住了"的假安全感比不提供更糟。
public enum MosaicStyle: String, Sendable, CaseIterable {
    case pixelate, solid
}

public struct WatermarkSpec: Sendable, Equatable {
    public var text: String
    /// 九宫格锚点 0-8,行优先:0 左上、4 居中、8 右下。
    public var anchor: Int
    public var opacity: Double
    /// 字号相对画面宽度的比例。
    public var scale: Double

    public init(text: String, anchor: Int = 8, opacity: Double = 0.45, scale: Double = 0.03) {
        self.text = text
        self.anchor = anchor
        self.opacity = opacity
        self.scale = scale
    }
}

/// 纯几何,不碰像素——KitCheck 能在无渲染环境里自检。
public enum EditGeometry {
    /// 裁剪矩形夹进 0-1 并保证最小边长(防手一抖裁出零面积)。
    public static func clampCrop(_ r: CGRect, minSide: Double = 0.02) -> CGRect {
        var x = max(0, min(r.origin.x, 1 - minSide))
        var y = max(0, min(r.origin.y, 1 - minSide))
        var w = max(minSide, r.width)
        var h = max(minSide, r.height)
        if x + w > 1 { w = 1 - x }
        if y + h > 1 { h = 1 - y }
        // 夹宽高后仍可能低于最小边(rect 本来就贴边),往回挪原点补足。
        if w < minSide { w = minSide; x = 1 - w }
        if h < minSide { h = minSide; y = 1 - h }
        return CGRect(x: x, y: y, width: w, height: h)
    }

    /// 顺时针转 90° 后的尺寸。
    public static func rotatedSize(_ s: CGSize, quarters: Int) -> CGSize {
        (quarters % 2 == 0) ? s : CGSize(width: s.height, height: s.width)
    }

    /// 归一化点随画面顺时针转 90°:(x,y) → (1-y, x)。
    public static func rotateQuarterCW(_ p: CGPoint) -> CGPoint {
        CGPoint(x: 1 - p.y, y: p.x)
    }

    /// 归一化矩形随画面顺时针转 90°。
    public static func rotateQuarterCW(_ r: CGRect) -> CGRect {
        // 转角点再取包围盒:旋转后 origin 换角,直接转四角最不易错。
        let a = rotateQuarterCW(CGPoint(x: r.minX, y: r.minY))
        let b = rotateQuarterCW(CGPoint(x: r.maxX, y: r.maxY))
        return CGRect(x: min(a.x, b.x), y: min(a.y, b.y),
                      width: abs(a.x - b.x), height: abs(a.y - b.y))
    }

    /// 把整份配方随画面顺时针转 90°(笔迹与裁剪框跟着画面走,半径归一化
    /// 到对角线所以不变)。
    public static func rotateSpecCW(_ spec: EditSpec) -> EditSpec {
        var s = spec
        s.rotationQuarters = (s.rotationQuarters + 1) % 4
        s.crop = s.crop.map(rotateQuarterCW)
        s.strokes = s.strokes.map { MosaicStroke(points: $0.points.map(rotateQuarterCW), radius: $0.radius) }
        return s
    }

    /// 九宫格锚点 → 水印左下角原点(像素坐标,CG 坐标系 y 向上)。
    public static func watermarkOrigin(anchor: Int, textSize: CGSize,
                                       canvas: CGSize, margin: Double) -> CGPoint {
        let col = Double(max(0, min(anchor, 8)) % 3)
        let row = Double(max(0, min(anchor, 8)) / 3)   // 0 顶 1 中 2 底
        let x: Double = switch col {
        case 0: margin
        case 1: (canvas.width - textSize.width) / 2
        default: canvas.width - textSize.width - margin
        }
        // CG y 向上:九宫格第 0 行是视觉顶部 → 高 y。
        let y: Double = switch row {
        case 0: canvas.height - textSize.height - margin
        case 1: (canvas.height - textSize.height) / 2
        default: margin
        }
        return CGPoint(x: x, y: y)
    }

    /// 拖拽版比例约束:以锚点(拖拽的固定对角)为不动点,朝光标方向按像素
    /// 比定框。中心重锚版(下面那个)只适合比例下拉框——拖角时用它会让
    /// "固定角"持续漂移。minSide 兜底两轴同乘,比例不破;顶到画布边同样
    /// 等比缩回。
    public static func applyRatio(anchor: CGPoint, cursor: CGPoint, pixelRatio: Double,
                                  canvas: CGSize, minSide: Double = 0.02) -> CGRect {
        guard pixelRatio > 0, canvas.width > 0, canvas.height > 0 else {
            return clampCrop(CGRect(x: min(anchor.x, cursor.x), y: min(anchor.y, cursor.y),
                                    width: abs(anchor.x - cursor.x), height: abs(anchor.y - cursor.y)))
        }
        var pw = abs(cursor.x - anchor.x) * canvas.width
        var ph = abs(cursor.y - anchor.y) * canvas.height
        if ph * pixelRatio > pw { ph = pw / pixelRatio } else { pw = ph * pixelRatio }
        let k = max(1, minSide * canvas.width / max(pw, 1e-9),
                    minSide * canvas.height / max(ph, 1e-9))
        pw *= k
        ph *= k
        var w = min(pw / canvas.width, 1)
        var h = min(ph / canvas.height, 1)
        // 越过画布边:两轴同乘缩回——归一化宽高等比缩,像素比不变。
        var shrink = 1.0
        if cursor.x < anchor.x { shrink = min(shrink, anchor.x / max(w, 1e-9)) }
        else { shrink = min(shrink, (1 - anchor.x) / max(w, 1e-9)) }
        if cursor.y < anchor.y { shrink = min(shrink, anchor.y / max(h, 1e-9)) }
        else { shrink = min(shrink, (1 - anchor.y) / max(h, 1e-9)) }
        if shrink < 1 {
            w *= shrink
            h *= shrink
        }
        let x = cursor.x >= anchor.x ? anchor.x : anchor.x - w
        let y = cursor.y >= anchor.y ? anchor.y : anchor.y - h
        return CGRect(x: max(0, min(x, 1 - w)), y: max(0, min(y, 1 - h)), width: w, height: h)
    }

    /// 以中心为锚把矩形调整为给定宽高比(ratio = w/h),并夹回 0-1。
    /// 注意归一化坐标的比例要预先按画布像素换算——调用方传像素比。
    public static func applyRatio(_ r: CGRect, pixelRatio: Double, canvas: CGSize) -> CGRect {
        guard pixelRatio > 0, canvas.width > 0, canvas.height > 0 else { return r }
        // 换到像素域求形状,再归一化回去,避免归一化域里比例失真。
        var pw = r.width * canvas.width
        var ph = r.height * canvas.height
        if pw / ph > pixelRatio {
            pw = ph * pixelRatio
        } else {
            ph = pw / pixelRatio
        }
        let cx = (r.midX) * canvas.width
        let cy = (r.midY) * canvas.height
        let out = CGRect(x: (cx - pw / 2) / canvas.width,
                         y: (cy - ph / 2) / canvas.height,
                         width: pw / canvas.width,
                         height: ph / canvas.height)
        return clampCrop(out)
    }
}

/// 渲染:配方 + 源文件 → 编辑产物(临时文件)。
public enum ImageEdit {
    /// 单幅像素上限:80MP 全尺寸 RGBA 上下文一步就是 320MB,叠加各渲染步
    /// 骤峰值破 GB——编辑器不是给航拍拼接图用的。
    public static let maxEditPixels = 80_000_000

    public enum Editability: Sendable {
        case ok, animated, tooLarge, unreadable
    }

    /// 编辑器能处理的输入。动图不行——逐帧编辑是另一码事,静默取首帧会
    /// 丢动画,不如明说不支持;损坏文件与超大图各有各的原因,分开报,
    /// 别把一切失败都说成"动图"。
    public static func editability(_ url: URL) -> Editability {
        guard let src = CGImageSourceCreateWithURL(url as CFURL, nil),
              CGImageSourceGetCount(src) >= 1 else { return .unreadable }
        if CGImageSourceGetCount(src) > 1 { return .animated }
        guard let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
              let pw = props[kCGImagePropertyPixelWidth] as? Int,
              let ph = props[kCGImagePropertyPixelHeight] as? Int else { return .unreadable }
        if pw * ph > maxEditPixels { return .tooLarge }
        return .ok
    }

    public static func editable(_ url: URL) -> Bool { editability(url) == .ok }

    /// 头部读像素尺寸,不解码。编辑器用它当画布几何基准——不能等预览
    /// 渲染完才有基准,旋转窗口期的比例/手势会用错画布。
    public static func pixelSize(of url: URL) -> CGSize? {
        guard let src = CGImageSourceCreateWithURL(url as CFURL, nil),
              let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
              let pw = props[kCGImagePropertyPixelWidth] as? Int,
              let ph = props[kCGImagePropertyPixelHeight] as? Int else { return nil }
        return CGSize(width: pw, height: ph)
    }

    /// 水印文字的光学尺寸——预览与渲染共用这一个度量,谁都别估算
    /// (0.62em/字那种估法对 CJK 偏小近四成)。
    public static func watermarkTextSize(_ text: String, fontSize: Double) -> CGSize {
        let font = CTFontCreateWithName("Helvetica-Bold" as CFString, fontSize, nil)
        let line = CTLineCreateWithAttributedString(NSAttributedString(
            string: text,
            attributes: [NSAttributedString.Key(kCTFontAttributeName as String): font]))
        return CTLineGetBoundsWithOptions(line, .useOpticalBounds).size
    }

    /// 编辑器画布用的预览:降采样解码 → 旋转 → 马赛克,不裁剪不打水印
    /// (那两样由画布叠加层所见即所得)。笔刷半径归一化到对角线,降采样
    /// 不改变含义,预览与成品逐像素同构。
    public static func preview(source: URL, spec: EditSpec, maxPixel: Int = 1600) -> CGImage? {
        guard let src = CGImageSourceCreateWithURL(source as CFURL, nil),
              CGImageSourceGetCount(src) == 1,
              let base = CGImageSourceCreateThumbnailAtIndex(src, 0, [
                  kCGImageSourceCreateThumbnailFromImageAlways: true,
                  kCGImageSourceThumbnailMaxPixelSize: maxPixel,
                  kCGImageSourceCreateThumbnailWithTransform: true,
              ] as CFDictionary) else { return nil }
        var img = base
        if spec.rotationQuarters % 4 != 0 {
            guard let r = rotate(img, quarters: spec.rotationQuarters) else { return nil }
            img = r
        }
        if !spec.strokes.isEmpty {
            guard let m = mosaic(img, strokes: spec.strokes, style: spec.mosaicStyle) else { return nil }
            img = m
        }
        return img
    }

    /// 渲染到独立临时目录,文件名保留源名(上传以 lastPathComponent 作
    /// 展示名);格式:JPEG 源出 JPEG,其余出 PNG——ImageIO 写不了 WebP,
    /// 而服务端反正会按用户设置重新编码,这里保真优先。
    public static func render(source: URL, spec: EditSpec) -> URL? {
        guard spec.hasEdits else { return nil }
        guard let src = CGImageSourceCreateWithURL(source as CFURL, nil),
              CGImageSourceGetCount(src) == 1 else { return nil }
        // 全尺寸解码并顺手把 EXIF 方向烙进像素(标签活不过重绘)。
        guard let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
              let pw = props[kCGImagePropertyPixelWidth] as? Int,
              let ph = props[kCGImagePropertyPixelHeight] as? Int,
              pw * ph <= maxEditPixels,
              let base = CGImageSourceCreateThumbnailAtIndex(src, 0, [
                  kCGImageSourceCreateThumbnailFromImageAlways: true,
                  kCGImageSourceThumbnailMaxPixelSize: max(pw, ph),
                  kCGImageSourceCreateThumbnailWithTransform: true,
              ] as CFDictionary) else { return nil }

        var img: CGImage = base
        if spec.rotationQuarters % 4 != 0 {
            guard let r = rotate(img, quarters: spec.rotationQuarters) else { return nil }
            img = r
        }
        if !spec.strokes.isEmpty {
            guard let m = mosaic(img, strokes: spec.strokes, style: spec.mosaicStyle) else { return nil }
            img = m
        }
        if let crop = spec.crop {
            let c = EditGeometry.clampCrop(crop)
            let px = CGRect(x: (c.origin.x * Double(img.width)).rounded(),
                            y: (c.origin.y * Double(img.height)).rounded(),
                            width: (c.width * Double(img.width)).rounded(),
                            height: (c.height * Double(img.height)).rounded())
            guard px.width >= 1, px.height >= 1, let cropped = img.cropping(to: px) else { return nil }
            img = cropped
        }
        if let wm = spec.watermark, !wm.text.isEmpty {
            guard let w = watermark(img, spec: wm) else { return nil }
            img = w
        }

        let isJPEG = CGImageSourceGetType(src).map { UTType($0 as String)?.conforms(to: .jpeg) ?? false } ?? false
        let ext = isJPEG ? "jpg" : "png"
        let uti = isJPEG ? UTType.jpeg.identifier : UTType.png.identifier
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("openimg-edit-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let name = source.deletingPathExtension().lastPathComponent
        let out = dir.appendingPathComponent(name).appendingPathExtension(ext)
        guard let dst = CGImageDestinationCreateWithURL(out as CFURL, uti as CFString, 1, nil) else { return nil }
        CGImageDestinationAddImage(dst, img, [
            kCGImageDestinationLossyCompressionQuality: 0.92,
        ] as CFDictionary)
        guard CGImageDestinationFinalize(dst) else {
            try? FileManager.default.removeItem(at: dir)
            return nil
        }
        return out
    }

    // MARK: - 渲染步骤

    private static func context(_ w: Int, _ h: Int) -> CGContext? {
        CGContext(data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                  space: CGColorSpace(name: CGColorSpace.sRGB)!,
                  bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
    }

    private static func rotate(_ img: CGImage, quarters: Int) -> CGImage? {
        let q = ((quarters % 4) + 4) % 4
        if q == 0 { return img }
        let w = img.width, h = img.height
        let out = EditGeometry.rotatedSize(CGSize(width: w, height: h), quarters: q)
        guard let ctx = context(Int(out.width), Int(out.height)) else { return nil }
        ctx.translateBy(x: out.width / 2, y: out.height / 2)
        // CG 坐标系 y 向上,顺时针(视觉)= 负角度。
        ctx.rotate(by: -Double(q) * .pi / 2)
        ctx.draw(img, in: CGRect(x: -Double(w) / 2, y: -Double(h) / 2,
                                 width: Double(w), height: Double(h)))
        return ctx.makeImage()
    }

    private static func mosaic(_ img: CGImage, strokes: [MosaicStroke], style: MosaicStyle) -> CGImage? {
        let w = img.width, h = img.height
        let diag = (Double(w * w + h * h)).squareRoot()
        guard let ctx = context(w, h) else { return nil }
        ctx.draw(img, in: CGRect(x: 0, y: 0, width: w, height: h))

        // 笔迹归一化坐标 y 向下(视图习惯),CG y 向上——翻转。
        let path = CGMutablePath()
        for s in strokes {
            guard let first = s.points.first else { continue }
            let lineWidth = max(2, s.radius * 2 * diag)
            let sub = CGMutablePath()
            sub.move(to: CGPoint(x: first.x * Double(w), y: (1 - first.y) * Double(h)))
            if s.points.count == 1 {
                // 单点也要着色:补一段极短线,strokingWithWidth 才有面积。
                sub.addLine(to: CGPoint(x: first.x * Double(w) + 0.1, y: (1 - first.y) * Double(h)))
            } else {
                for p in s.points.dropFirst() {
                    sub.addLine(to: CGPoint(x: p.x * Double(w), y: (1 - p.y) * Double(h)))
                }
            }
            path.addPath(sub.copy(strokingWithWidth: lineWidth, lineCap: .round,
                                  lineJoin: .round, miterLimit: 10))
        }
        guard !path.isEmpty else { return ctx.makeImage() }

        ctx.saveGState()
        ctx.addPath(path)
        ctx.clip()
        switch style {
        case .solid:
            ctx.setFillColor(CGColor(gray: 0, alpha: 1))
            ctx.fill(CGRect(x: 0, y: 0, width: w, height: h))
        case .pixelate:
            // 缩小再放大、关插值 = 经典像素化。格子随图幅走,截图上肉眼
            // 明确不可读。
            let cell = max(8.0, Double(max(w, h)) / 80)
            let sw = max(1, Int((Double(w) / cell).rounded())), sh = max(1, Int((Double(h) / cell).rounded()))
            guard let small = context(sw, sh) else { ctx.restoreGState(); return nil }
            small.interpolationQuality = .low
            small.draw(img, in: CGRect(x: 0, y: 0, width: sw, height: sh))
            guard let tiny = small.makeImage() else { ctx.restoreGState(); return nil }
            ctx.interpolationQuality = .none
            ctx.draw(tiny, in: CGRect(x: 0, y: 0, width: w, height: h))
        }
        ctx.restoreGState()
        return ctx.makeImage()
    }

    private static func watermark(_ img: CGImage, spec: WatermarkSpec) -> CGImage? {
        let w = img.width, h = img.height
        guard let ctx = context(w, h) else { return nil }
        ctx.draw(img, in: CGRect(x: 0, y: 0, width: w, height: h))

        let fontSize = max(9, spec.scale * Double(w))
        let font = CTFontCreateWithName("Helvetica-Bold" as CFString, fontSize, nil)
        let alpha = max(0.05, min(1, spec.opacity))

        func line(_ color: CGColor) -> CTLine {
            let attrs: [NSAttributedString.Key: Any] = [
                NSAttributedString.Key(kCTFontAttributeName as String): font,
                NSAttributedString.Key(kCTForegroundColorAttributeName as String): color,
            ]
            return CTLineCreateWithAttributedString(
                NSAttributedString(string: spec.text, attributes: attrs))
        }

        let white = line(CGColor(gray: 1, alpha: alpha))
        let bounds = CTLineGetBoundsWithOptions(white, .useOpticalBounds)
        let origin = EditGeometry.watermarkOrigin(
            anchor: spec.anchor,
            textSize: bounds.size,
            canvas: CGSize(width: w, height: h),
            margin: fontSize * 0.8)

        // CTLineDraw 的 textPosition 是基线起点,不是外框左下角;补偿光学
        // 边界原点,外框才恰好落在 watermarkOrigin 算出的位置——否则顶部
        // 锚点会下沉约半个字号,降部字符在底部锚压进边距。
        let pen = CGPoint(x: origin.x - bounds.origin.x, y: origin.y - bounds.origin.y)
        // 影子再正字:白字在亮背景上会隐形,一枚低透明度黑影保住可读性。
        let shadow = line(CGColor(gray: 0, alpha: alpha * 0.7))
        let off = max(1, fontSize / 24)
        ctx.textPosition = CGPoint(x: pen.x + off, y: pen.y - off)
        CTLineDraw(shadow, ctx)
        ctx.textPosition = pen
        CTLineDraw(white, ctx)
        return ctx.makeImage()
    }
}
