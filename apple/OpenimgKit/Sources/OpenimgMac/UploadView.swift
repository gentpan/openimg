import SwiftUI
import OpenimgKit
import UniformTypeIdentifiers

struct UploadView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 16) {
            if model.queue.isEmpty {
                // The drop target takes the room rather than floating in it.
                // A small dashed box centred in a large empty page reads as an
                // afterthought; on this screen it is the whole point.
                dropZone
                conversion
                formatRow
                limits
            } else {
                batchHeader
                queueList
                // The drop zone stays reachable while a batch is on screen, so
                // adding more files does not mean clearing the list first.
                dropZone.frame(maxHeight: 76)
                formatRow
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(.horizontal, 26)
        .padding(.bottom, 18)
    }

    // MARK: - Drop zone

    private var dropZone: some View {
        RoundedRectangle(cornerRadius: 16, style: .continuous)
            .fill(model.dropping ? Color.brand.opacity(0.14) : .white.opacity(0.035))
            .overlay(
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .strokeBorder(model.dropping ? Color.brand : .white.opacity(0.14),
                                  style: StrokeStyle(lineWidth: model.dropping ? 2 : 1.2, dash: [7, 5]))
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .overlay { zoneContent }
            .contentShape(Rectangle())
            .onTapGesture {
                guard !model.uploading else { return }
                Task { await model.pickAndUpload() }
            }
            .onDrop(of: [.fileURL], isTargeted: $model.dropping) { providers in
                // Dropped folders expand the same way a picked one does.
                Task { await model.upload(model.expand(await urls(from: providers))) }
                return true
            }
            .animation(.easeOut(duration: 0.15), value: model.dropping)
    }

    @ViewBuilder
    private var zoneContent: some View {
        if model.queue.isEmpty {
            VStack(spacing: 12) {
                Image(systemName: "arrow.up.doc")
                    .font(.system(size: 38, weight: .light))
                    .foregroundStyle(Color.brand)
                    .frame(width: 88, height: 88)
                    .background(Circle().fill(Color.brand.opacity(0.12)))
                Text("把图片或文件夹拖到这里").font(.title2.weight(.medium))
                Text("或点击选择，文件夹会自动展开")
                    .font(.callout).foregroundStyle(.secondary)
            }
        } else {
            Label("继续添加", systemImage: "plus")
                .font(.callout).foregroundStyle(.secondary)
        }
    }

    // MARK: - Batch

    private var batchHeader: some View {
        VStack(spacing: 9) {
            HStack {
                if model.uploading {
                    ProgressView().controlSize(.small)
                    Text("上传中 \(model.queue.filter { $0.state == .done }.count) / \(model.queue.count)")
                        .font(.callout)
                } else {
                    let ok = model.queue.filter { $0.state == .done }.count
                    let bad = model.queue.count - ok
                    Image(systemName: bad == 0 ? "checkmark.circle.fill" : "exclamationmark.circle.fill")
                        .foregroundStyle(bad == 0 ? .green : .orange)
                    Text(bad == 0 ? "\(ok) 张全部完成" : "\(ok) 张完成，\(bad) 张未成功")
                        .font(.callout)
                }
                Spacer()
                Button(model.uploading ? "上传中…" : "清空列表") { model.clearQueue() }
                    .buttonStyle(QuietButton())
                    .disabled(model.uploading)
            }
            ProgressBar(tint: .brand, value: model.batchProgress)
                .frame(height: 5)
        }
        .padding(14)
        .panelSurface(12)
        .padding(.top, 4)
    }

    private var queueList: some View {
        ScrollView {
            VStack(spacing: 6) {
                ForEach(model.queue) { QueueRow(item: $0, bytes: model.bytes) }
            }
        }
        .frame(maxHeight: .infinity)
    }

    // MARK: - Bits

    private var formatRow: some View {
        VStack(spacing: 7) {
            Text("上传后复制").font(.caption).foregroundStyle(.secondary)
            PillRow(items: LinkFormat.allCases, label: \.label, selection: $model.linkFormat)
        }
    }

    /// Conversion settings, here rather than only in settings: this is where
    /// the user is about to upload, and it is the moment they care whether the
    /// file gets re-encoded. The same values live on the account, so a change
    /// made here shows up on the website too.
    private var conversion: some View {
        // Labels sit above their own control rather than spanning the row: at
        // this width a leading label and a trailing hint end up at opposite
        // edges of the window with the control stranded between them.
        VStack(spacing: 12) {
            group("处理方式", model.uploadMode.detail) {
                PillRow(items: UploadMode.allCases, label: \.label,
                        selection: preference($model.uploadMode))
            }
            HStack(alignment: .top, spacing: 22) {
                group("衍生格式", nil) {
                    PillRow(items: VariantFormat.allCases, label: \.label,
                            selection: preference($model.variantFormat))
                }
                group("最大宽度", nil) {
                    PillRow(items: allowedMaxWidths.map(Width.init), label: \.label,
                            selection: widthBinding)
                }
            }
        }
        .frame(maxWidth: 560)
    }

    private func group<C: View>(_ title: String, _ hint: String?,
                                @ViewBuilder control: () -> C) -> some View {
        VStack(spacing: 5) {
            Text(title).font(.caption).foregroundStyle(.secondary)
            control()
            if let hint {
                Text(hint).font(.caption2).foregroundStyle(.tertiary)
            }
        }
    }

    /// Saves on change. Wrapping the binding keeps the write next to the value
    /// it belongs to, instead of an onChange hanging off each control.
    private func preference<T>(_ b: Binding<T>) -> Binding<T> {
        Binding(get: { b.wrappedValue },
                set: { b.wrappedValue = $0; Task { await model.savePreferences() } })
    }

    private var widthBinding: Binding<Width> {
        Binding(get: { Width(model.maxImageWidth) },
                set: { model.maxImageWidth = $0.px; Task { await model.savePreferences() } })
    }

    struct Width: Hashable, Identifiable {
        let px: Int
        init(_ px: Int) { self.px = px }
        var id: Int { px }
        var label: String { px == 0 ? "不限" : "\(px)" }
    }

    @ViewBuilder
    private var limits: some View {
        if let t = model.quota?.tier {
            HStack(spacing: 18) {
                if t.dailyUploadCount > 0, let used = model.quota?.uploadsToday {
                    let left = max(0, t.dailyUploadCount - used)
                    // Before the upload rather than after the failure: unlike
                    // the per-minute limit, the daily count does not clear
                    // until tomorrow, so it is not something to retry through.
                    stat("今日剩余", "\(left) 张", warn: left <= 5)
                }
                stat("单文件上限", model.bytes(t.maxFileSize))
                stat("支持格式", t.allowedFormats.prefix(4).joined(separator: " · ").uppercased())
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 12)
            .panelSurface(12)
        }
    }

    private func stat(_ k: String, _ v: String, warn: Bool = false) -> some View {
        VStack(spacing: 2) {
            Text(k).font(.caption2).foregroundStyle(.tertiary)
            Text(v).font(.callout.weight(.medium)).foregroundStyle(warn ? .orange : .primary)
        }
    }

    /// NSItemProvider returns a file URL as its Data representation; decoding it
    /// any other way silently yields nil for every drop.
    private func urls(from providers: [NSItemProvider]) async -> [URL] {
        var out: [URL] = []
        for p in providers {
            guard let item = try? await p.loadItem(forTypeIdentifier: UTType.fileURL.identifier),
                  let data = item as? Data,
                  let url = URL(dataRepresentation: data, relativeTo: nil) else { continue }
            out.append(url)
        }
        return out
    }
}

// MARK: - Row

private struct QueueRow: View {
    let item: UploadItem
    let bytes: (Int64) -> String

    var body: some View {
        HStack(spacing: 11) {
            icon
            VStack(alignment: .leading, spacing: 3) {
                HStack(spacing: 6) {
                    Text(item.name).font(.callout).lineLimit(1).truncationMode(.middle)
                    if let sent = item.sentBytes, item.size > 0 {
                        Text("已缩至 \(Int(Double(sent) / Double(item.size) * 100))%")
                            .font(.system(size: 9, weight: .medium))
                            .foregroundStyle(Color.brand)
                            .padding(.horizontal, 5).padding(.vertical, 1.5)
                            .background(Capsule().fill(Color.brand.opacity(0.16)))
                    }
                    if item.deduplicated {
                        Text("秒传")
                            .font(.system(size: 9, weight: .medium))
                            .foregroundStyle(Color.success)
                            .padding(.horizontal, 5).padding(.vertical, 1.5)
                            .background(Capsule().fill(.green.opacity(0.16)))
                    }
                    Spacer(minLength: 0)
                    Text(detail).font(.caption2).foregroundStyle(.secondary).monospacedDigit()
                }
                if case .uploading = item.state {
                    ProgressBar(tint: .brand, value: item.progress).frame(height: 3)
                } else if case .failed(let why) = item.state {
                    Text(why).font(.caption2).foregroundStyle(.orange).lineLimit(1)
                }
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .panelSurface(10)
    }

    private var detail: String {
        switch item.state {
        case .queued: "等待中"
        case .uploading: "\(Int(item.progress * 100))%"
        case .done: item.size > 0 ? bytes(item.size) : "完成"
        case .failed: ""
        }
    }

    @ViewBuilder
    private var icon: some View {
        switch item.state {
        case .queued:
            Image(systemName: "clock").foregroundStyle(.tertiary)
        case .uploading:
            ProgressView().controlSize(.small).scaleEffect(0.7).frame(width: 16)
        case .done:
            Image(systemName: "checkmark.circle.fill").foregroundStyle(Color.brand)
        case .failed:
            Image(systemName: "exclamationmark.circle.fill").foregroundStyle(.orange)
        }
    }
}

/// One progress bar shape, so the batch bar and the per-file bars match.
struct ProgressBar: View {
    /// Which scale the bar belongs to.
    ///
    /// The same bar serves two different questions. Upload progress is "how
    /// far along is this task" — navigation's business, so violet. Check-in
    /// progress is "how many days have I banked" — a quantity, so green, which
    /// is the default because that is the more common case here.
    var tint: Color = .accent

    let value: Double
    var body: some View {
        GeometryReader { geo in
            ZStack(alignment: .leading) {
                Capsule().fill(.white.opacity(0.10))
                Capsule().fill(tint)
                    .frame(width: max(2, geo.size.width * min(1, max(0, value))))
                    .animation(.easeOut(duration: 0.2), value: value)
            }
        }
    }
}
