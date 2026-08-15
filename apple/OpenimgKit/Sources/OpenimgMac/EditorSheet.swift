import SwiftUI
import AppKit
import OpenimgKit

/// 编辑入口的载体:sheet(item:) 需要 Identifiable。
struct EditTarget: Identifiable {
    let id = UUID()
    let url: URL
}

extension AppModel {
    /// 挑一张图进编辑器;动图明说不支持,而不是静默取首帧丢动画。
    func pickAndEdit() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = false
        panel.allowedContentTypes = [.image]
        panel.prompt = "编辑"
        guard panel.runModal() == .OK, let url = panel.url else { return }
        openEditor(url)
    }

    func openEditor(_ url: URL) {
        switch ImageEdit.editability(url) {
        case .ok:
            editTarget = EditTarget(url: url)
        case .animated:
            announce("动图不支持编辑，可直接上传")
        case .tooLarge:
            announce("图片超过 8000 万像素，暂不支持编辑，可直接上传")
        case .unreadable:
            announce("无法读取此图片（文件可能已损坏）")
        }
    }

    /// 拖放路由:开了"单张先编辑"且**拖的就是单个文件**时进编辑器——按
    /// 原始拖放物判定,不按 expand 展开后的数量,否则"恰好含一张图的文件
    /// 夹"也会弹编辑器,和开关文案矛盾。
    func handleDrop(_ urls: [URL]) async {
        if editOnDrop, urls.count == 1, let one = urls.first,
           (try? one.resourceValues(forKeys: [.isDirectoryKey]).isDirectory) != true,
           ImageEdit.editable(one) {
            editTarget = EditTarget(url: one)
            return
        }
        await upload(expand(urls))
    }

    /// 编辑确认:渲染进现有上传队列。没有实际编辑就传原件——不为"点了
    /// 编辑"这个动作本身付一次重编码。
    ///
    /// 渲染失败**绝不回落上传原件**:马赛克的用途就是涂掉隐私,失败时把
    /// 未打码的原图静默传上去是这条管线最坏的失败模式。保持编辑器打开
    /// (笔迹还在),播报错误让用户重试。
    func confirmEdit(source: URL, spec: EditSpec) {
        Task {
            guard spec.hasEdits else {
                editTarget = nil
                await upload([source])
                return
            }
            editSubmitting = true
            defer { editSubmitting = false }
            let rendered = await Task.detached { ImageEdit.render(source: source, spec: spec) }.value
            guard let out = rendered else {
                announce("编辑渲染失败，未上传——请重试或取消")
                return
            }
            editTarget = nil
            await upload([out])
            try? FileManager.default.removeItem(at: out.deletingLastPathComponent())
        }
    }
}

/// 上传前编辑器:裁剪 / 马赛克 / 旋转 / 水印。
///
/// 预览 = Kit 渲染的降采样底图(旋转+马赛克) + 画布叠加层(裁剪框、水印
/// 文字)——底图与成品走同一段渲染代码,所见即所得不靠两套绘制对齐。
struct EditorSheet: View {
    @ObservedObject var model: AppModel
    let source: URL

    @State private var spec = EditSpec()
    @State private var mode: Mode = .crop
    @State private var brush = 0.018            // 归一化到对角线
    @State private var wmOn = false
    @State private var preview: CGImage?
    /// 源图像素尺寸,头部读取——画布几何基准从它+旋转次数推导,不依赖
    /// 预览是否渲染完,旋转窗口期的比例/手势才不会用错画布。
    @State private var sourcePixelSize: CGSize = .zero
    @State private var activeStroke: [CGPoint] = []   // 进行中的笔迹(归一化)
    /// 已提交、底图还没带上的笔迹:保持不透明叠加,别让刚遮住的内容在
    /// 渲染空窗里裸露闪现。
    @State private var pendingStroke: [CGPoint] = []
    @State private var pendingBrush = 0.018
    @State private var cropDrag: CropDrag?
    @State private var rendering = false
    /// 预览代数:连点旋转时旧渲染结果可能后到,只认最新一代。
    @State private var previewGen = 0
    @State private var showCancelConfirm = false
    @State private var suppressRatioReapply = false

    private static let transposedRatio = ["4:3": "3:4", "3:4": "4:3", "16:9": "9:16", "9:16": "16:9"]

    enum Mode: String, CaseIterable {
        case crop = "裁剪", mosaic = "马赛克"
    }

    private struct CropDrag {
        enum Kind { case move, new, corner(Int) }   // corner: 0 左上 1 右上 2 左下 3 右下
        var kind: Kind
        var startRect: CGRect
        var startPoint: CGPoint     // 归一化
    }

    private struct RatioPreset: Identifiable {
        let id: String
        let ratio: Double?          // 像素比 w/h,nil = 自由
    }
    private static let ratios: [RatioPreset] = [
        .init(id: "自由", ratio: nil), .init(id: "1:1", ratio: 1),
        .init(id: "4:3", ratio: 4 / 3), .init(id: "16:9", ratio: 16 / 9),
        .init(id: "3:4", ratio: 3 / 4), .init(id: "9:16", ratio: 9 / 16),
    ]
    @State private var ratioID = "自由"

    var body: some View {
        VStack(spacing: 0) {
            toolbar
            Divider().overlay(Color.white.opacity(0.08))
            canvas
            Divider().overlay(Color.white.opacity(0.08))
            bottomBar
        }
        .frame(minWidth: 880, idealWidth: 980, minHeight: 620, idealHeight: 700)
        .background(Color.black.opacity(0.92))
        .task {
            sourcePixelSize = ImageEdit.pixelSize(of: source) ?? .zero
            await refreshPreview()
        }
        .onChange(of: mode) { _, _ in activeStroke = [] }
        .confirmationDialog("放弃当前编辑？", isPresented: $showCancelConfirm) {
            Button("放弃编辑", role: .destructive) { model.editTarget = nil }
            Button("继续编辑", role: .cancel) {}
        } message: {
            Text("裁剪、涂抹和旋转都会丢失。")
        }
    }

    // MARK: - 工具栏

    private var toolbar: some View {
        HStack(spacing: 14) {
            Picker("", selection: $mode) {
                ForEach(Mode.allCases, id: \.self) { Text($0.rawValue) }
            }
            .pickerStyle(.segmented)
            .frame(width: 170)

            if mode == .crop {
                Picker("比例", selection: $ratioID) {
                    ForEach(Self.ratios) { Text($0.id).tag($0.id) }
                }
                .frame(width: 130)
                .onChange(of: ratioID) { _, id in
                    // 旋转按钮转置比例时只换显示,框已被 rotateSpecCW 转好,
                    // 重套一遍反而会用错基准。
                    if suppressRatioReapply {
                        suppressRatioReapply = false
                        return
                    }
                    guard let ratio = Self.ratios.first(where: { $0.id == id })?.ratio else { return }
                    let base = spec.crop ?? CGRect(x: 0, y: 0, width: 1, height: 1)
                    spec.crop = EditGeometry.applyRatio(base, pixelRatio: ratio, canvas: rotatedPixelSize)
                }
                Button("清除裁剪") { spec.crop = nil; ratioID = "自由" }
                    .controlSize(.small)
                    .disabled(spec.crop == nil)
            }

            if mode == .mosaic {
                Picker("", selection: $spec.mosaicStyle) {
                    Text("像素化").tag(MosaicStyle.pixelate)
                    Text("纯色").tag(MosaicStyle.solid)
                }
                .pickerStyle(.segmented)
                .frame(width: 140)
                .onChange(of: spec.mosaicStyle) { _, _ in Task { await refreshPreview() } }
                Text("笔刷").font(.caption).foregroundStyle(.secondary)
                Slider(value: $brush, in: 0.006...0.06).frame(width: 110)
                Button("撤销一笔") {
                    _ = spec.strokes.popLast()
                    Task { await refreshPreview() }
                }
                .controlSize(.small)
                .keyboardShortcut("z", modifiers: .command)
                .disabled(spec.strokes.isEmpty)
            }

            Button {
                spec = EditGeometry.rotateSpecCW(spec)
                // 比例随画面转置(16:9 → 9:16),只换显示不重套框。
                if let t = Self.transposedRatio[ratioID] {
                    suppressRatioReapply = true
                    ratioID = t
                }
                Task { await refreshPreview() }
            } label: { Label("旋转", systemImage: "rotate.right") }
                .controlSize(.small)

            Toggle("水印", isOn: $wmOn)
                .toggleStyle(.checkbox)
                .disabled(model.watermarkSpec() == nil)
                .help(model.watermarkSpec() == nil
                      ? "先在 设置 → 水印 里填好水印文字" : "按设置里的样式加水印")

            Spacer()
            if rendering { ProgressView().controlSize(.small) }
            Text(source.lastPathComponent)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)
                .frame(maxWidth: 200)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
    }

    // MARK: - 画布

    private var canvas: some View {
        GeometryReader { geo in
            let fit = fittedRect(in: geo.size)
            ZStack {
                if let preview {
                    Image(decorative: preview, scale: 1)
                        .resizable()
                        .interpolation(.high)
                        .frame(width: fit.width, height: fit.height)
                        .position(x: fit.midX, y: fit.midY)
                } else {
                    ProgressView()
                }

                if mode == .mosaic, !activeStroke.isEmpty || !pendingStroke.isEmpty {
                    strokePreview(fit: fit)
                }
                if mode == .crop {
                    cropOverlay(fit: fit)
                }
                if wmOn, let wm = model.watermarkSpec() {
                    watermarkOverlay(wm, fit: fit)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .contentShape(Rectangle())
            .gesture(mode == .crop ? cropGesture(fit: fit) : nil)
            .gesture(mode == .mosaic ? mosaicGesture(fit: fit) : nil)
        }
        .padding(12)
    }

    /// 旋转后的像素尺寸(几何计算的画布基准)。从源尺寸+旋转次数推导而
    /// 不是取预览图尺寸——预览是异步渲染的,取它会让旋转后的窗口期用旧
    /// 朝向算比例和手势,错的裁剪框会写进 spec 持久污染。
    private var rotatedPixelSize: CGSize {
        guard sourcePixelSize.width > 0, sourcePixelSize.height > 0 else {
            guard let p = preview else { return CGSize(width: 1, height: 1) }
            return CGSize(width: p.width, height: p.height)
        }
        return EditGeometry.rotatedSize(sourcePixelSize, quarters: spec.rotationQuarters)
    }

    private func fittedRect(in avail: CGSize) -> CGRect {
        let ps = rotatedPixelSize
        guard ps.width > 0, ps.height > 0, avail.width > 0, avail.height > 0 else { return .zero }
        let scale = min(avail.width / ps.width, avail.height / ps.height)
        let w = ps.width * scale, h = ps.height * scale
        return CGRect(x: (avail.width - w) / 2, y: (avail.height - h) / 2, width: w, height: h)
    }

    private func normalize(_ p: CGPoint, in fit: CGRect) -> CGPoint {
        CGPoint(x: max(0, min(1, (p.x - fit.minX) / fit.width)),
                y: max(0, min(1, (p.y - fit.minY) / fit.height)))
    }

    // MARK: - 马赛克

    private func mosaicGesture(fit: CGRect) -> some Gesture {
        DragGesture(minimumDistance: 0)
            .onChanged { v in
                let p = normalize(v.location, in: fit)
                // 点距稀疏化:密集采样徒增路径长度不增覆盖。
                if let last = activeStroke.last {
                    let dx = p.x - last.x, dy = p.y - last.y
                    if (dx * dx + dy * dy).squareRoot() < brush / 3 { return }
                }
                activeStroke.append(p)
            }
            .onEnded { _ in
                guard !activeStroke.isEmpty else { return }
                spec.strokes.append(MosaicStroke(points: activeStroke, radius: brush))
                // 交给 pendingStroke 顶住渲染空窗:立即清叠加线会让刚遮住
                // 的内容在底图带上马赛克前裸露闪现。
                pendingStroke = activeStroke
                pendingBrush = brush
                activeStroke = []
                Task { await refreshPreview() }
            }
    }

    private func strokePath(_ points: [CGPoint], fit: CGRect) -> Path {
        Path { p in
            guard let first = points.first else { return }
            p.move(to: CGPoint(x: fit.minX + first.x * fit.width, y: fit.minY + first.y * fit.height))
            for pt in points.dropFirst() {
                p.addLine(to: CGPoint(x: fit.minX + pt.x * fit.width, y: fit.minY + pt.y * fit.height))
            }
        }
    }

    @ViewBuilder
    private func strokePreview(fit: CGRect) -> some View {
        let diag = (fit.width * fit.width + fit.height * fit.height).squareRoot()
        // 进行中笔迹:纯色模式直接给成品同款纯黑,像素化给覆盖性的灰——
        // 半透明品牌色会给"看起来没遮住"的错觉。
        let activeColor: Color = spec.mosaicStyle == .solid
            ? .black.opacity(0.9) : Color(white: 0.5, opacity: 0.9)
        let pendingColor: Color = spec.mosaicStyle == .solid
            ? .black : Color(white: 0.5, opacity: 0.95)
        if !pendingStroke.isEmpty {
            strokePath(pendingStroke, fit: fit)
                .stroke(pendingColor,
                        style: StrokeStyle(lineWidth: max(2, pendingBrush * 2 * diag),
                                           lineCap: .round, lineJoin: .round))
                .allowsHitTesting(false)
        }
        if !activeStroke.isEmpty {
            strokePath(activeStroke, fit: fit)
                .stroke(activeColor,
                        style: StrokeStyle(lineWidth: max(2, brush * 2 * diag),
                                           lineCap: .round, lineJoin: .round))
                .allowsHitTesting(false)
        }
    }

    // MARK: - 裁剪

    private func cropRect(in fit: CGRect) -> CGRect {
        let c = spec.crop ?? CGRect(x: 0, y: 0, width: 1, height: 1)
        return CGRect(x: fit.minX + c.minX * fit.width, y: fit.minY + c.minY * fit.height,
                      width: c.width * fit.width, height: c.height * fit.height)
    }

    @ViewBuilder
    private func cropOverlay(fit: CGRect) -> some View {
        let r = cropRect(in: fit)
        // 框外压暗:even-odd 挖洞。
        Path { p in
            p.addRect(CGRect(origin: .zero, size: CGSize(width: 10_000, height: 10_000)))
            p.addRect(r)
        }
        .fill(Color.black.opacity(0.55), style: FillStyle(eoFill: true))
        .allowsHitTesting(false)

        Rectangle()
            .strokeBorder(Color.brand, lineWidth: 1.5)
            .frame(width: r.width, height: r.height)
            .position(x: r.midX, y: r.midY)
            .allowsHitTesting(false)

        ForEach(0..<4, id: \.self) { i in
            let pt = cornerPoint(i, of: r)
            Circle()
                .fill(Color.brand)
                .frame(width: 11, height: 11)
                .position(pt)
                .allowsHitTesting(false)
        }
    }

    private func cornerPoint(_ i: Int, of r: CGRect) -> CGPoint {
        switch i {
        case 0: CGPoint(x: r.minX, y: r.minY)
        case 1: CGPoint(x: r.maxX, y: r.minY)
        case 2: CGPoint(x: r.minX, y: r.maxY)
        default: CGPoint(x: r.maxX, y: r.maxY)
        }
    }

    private func cropGesture(fit: CGRect) -> some Gesture {
        DragGesture(minimumDistance: 0)
            .onChanged { v in
                let p = normalize(v.location, in: fit)
                if cropDrag == nil {
                    cropDrag = beginCropDrag(at: v.startLocation, fit: fit)
                }
                guard let drag = cropDrag else { return }
                let ratio = Self.ratios.first(where: { $0.id == ratioID })?.ratio
                switch drag.kind {
                case .move:
                    // 移动只平移不改形状——重套比例会让框在拖动中变形。
                    var next = drag.startRect.offsetBy(dx: p.x - drag.startPoint.x,
                                                       dy: p.y - drag.startPoint.y)
                    next.origin.x = max(0, min(next.origin.x, 1 - next.width))
                    next.origin.y = max(0, min(next.origin.y, 1 - next.height))
                    spec.crop = next
                case .new:
                    spec.crop = constrained(anchor: drag.startPoint, cursor: p, ratio: ratio)
                case .corner(let idx):
                    // 对角固定,拖动角走:0↔3、1↔2 互为对角。
                    let anchor = cornerNorm(3 - idx, of: drag.startRect)
                    spec.crop = constrained(anchor: anchor, cursor: p, ratio: ratio)
                }
            }
            .onEnded { _ in cropDrag = nil }
    }

    /// 比例约束用锚点版:固定角钉死、框贴光标;中心重锚版只配比例下拉框。
    private func constrained(anchor: CGPoint, cursor: CGPoint, ratio: Double?) -> CGRect {
        if let ratio {
            return EditGeometry.applyRatio(anchor: anchor, cursor: cursor,
                                           pixelRatio: ratio, canvas: rotatedPixelSize)
        }
        return EditGeometry.clampCrop(rectFrom(anchor, cursor))
    }

    private func beginCropDrag(at viewPoint: CGPoint, fit: CGRect) -> CropDrag {
        let start = normalize(viewPoint, in: fit)
        let current = spec.crop ?? CGRect(x: 0, y: 0, width: 1, height: 1)
        let r = cropRect(in: fit)
        for i in 0..<4 where hypot(viewPoint.x - cornerPoint(i, of: r).x,
                                   viewPoint.y - cornerPoint(i, of: r).y) < 14 {
            return CropDrag(kind: .corner(i), startRect: current, startPoint: start)
        }
        // 框内(除四角)整体都是移动,不留边缘死区——底栏文案就是这么承诺的。
        if spec.crop != nil, r.contains(viewPoint) {
            return CropDrag(kind: .move, startRect: current, startPoint: start)
        }
        return CropDrag(kind: .new, startRect: current, startPoint: start)
    }

    private func rectFrom(_ a: CGPoint, _ b: CGPoint) -> CGRect {
        CGRect(x: min(a.x, b.x), y: min(a.y, b.y), width: abs(a.x - b.x), height: abs(a.y - b.y))
    }

    private func cornerNorm(_ i: Int, of r: CGRect) -> CGPoint {
        switch i {
        case 0: CGPoint(x: r.minX, y: r.minY)
        case 1: CGPoint(x: r.maxX, y: r.minY)
        case 2: CGPoint(x: r.minX, y: r.maxY)
        default: CGPoint(x: r.maxX, y: r.maxY)
        }
    }

    // MARK: - 水印预览

    private func watermarkOverlay(_ wm: WatermarkSpec, fit: CGRect) -> some View {
        // 与渲染同一套几何:成品水印打在**裁剪后**的画布上,预览基准也必须
        // 是裁剪框(没有裁剪时即整幅)——否则裁剪+水印同开时位置和字号都
        // 是错的。字宽用与渲染同源的 CTLine 度量,不估算(0.62em/字对 CJK
        // 偏小近四成),字体也指定同款免得度量对不上。
        let box = cropRect(in: fit)
        let fontSize = max(6, wm.scale * box.width)
        let textSize = ImageEdit.watermarkTextSize(wm.text, fontSize: fontSize)
        let o = EditGeometry.watermarkOrigin(anchor: wm.anchor, textSize: textSize,
                                             canvas: box.size, margin: fontSize * 0.8)
        return Text(wm.text)
            .font(.custom("Helvetica-Bold", size: fontSize))
            .foregroundStyle(.white.opacity(wm.opacity))
            .shadow(color: .black.opacity(wm.opacity * 0.7), radius: 0, x: 1, y: 1)
            .position(x: box.minX + o.x + textSize.width / 2,
                      y: box.minY + (box.height - o.y) - textSize.height / 2)
            .allowsHitTesting(false)
    }

    // MARK: - 底栏

    private var bottomBar: some View {
        HStack(spacing: 10) {
            Button("重置") {
                spec = EditSpec()
                ratioID = "自由"
                wmOn = false
                pendingStroke = []
                activeStroke = []
                Task { await refreshPreview() }
            }
            .disabled(!spec.hasEdits && !wmOn)
            Spacer()
            if mode == .mosaic {
                Text("在需要遮盖的地方拖动涂抹").font(.caption).foregroundStyle(.tertiary)
            } else {
                Text("拖出裁剪框，四角可调，框内拖动移动").font(.caption).foregroundStyle(.tertiary)
            }
            Spacer()
            // macOS 惯例:取消紧贴默认按钮居右。有编辑时取消先确认——
            // Esc 一键蒸发掉涂了半天的马赛克太残忍。
            Button("取消") {
                if spec.hasEdits {
                    showCancelConfirm = true
                } else {
                    model.editTarget = nil
                }
            }
            .keyboardShortcut(.cancelAction)
            Button {
                var final = spec
                final.watermark = wmOn ? model.watermarkSpec() : nil
                model.confirmEdit(source: source, spec: final)
            } label: {
                if model.editSubmitting {
                    ProgressView().controlSize(.small)
                } else {
                    Text("上传")
                }
            }
            .keyboardShortcut(.defaultAction)
            .buttonStyle(.borderedProminent)
            .disabled(model.editSubmitting)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
    }

    // MARK: - 预览

    private func refreshPreview() async {
        // 代数守卫:连点旋转时两次渲染在 await 点交错,旧结果后到会把
        // preview 钉在错误朝向上。只认最新一代;旧代不清 rendering——
        // 最新一代还在渲染,由它收尾。
        previewGen += 1
        let gen = previewGen
        rendering = true
        var s = spec
        s.crop = nil
        s.watermark = nil
        let src = source
        let img = await Task.detached { ImageEdit.preview(source: src, spec: s) }.value
        guard gen == previewGen else { return }
        preview = img
        pendingStroke = []
        rendering = false
    }
}
