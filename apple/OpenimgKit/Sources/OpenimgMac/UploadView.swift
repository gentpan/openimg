import SwiftUI
import OpenimgKit
import UniformTypeIdentifiers

struct UploadView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 18) {
            Spacer(minLength: 0)
            dropZone
            formatRow
            limits
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(.horizontal, 28)
        .padding(.bottom, 20)
    }

    private var dropZone: some View {
        RoundedRectangle(cornerRadius: 16, style: .continuous)
            .fill(model.dropping ? Color.brand.opacity(0.12) : .white.opacity(0.035))
            .overlay(
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .strokeBorder(
                        model.dropping ? Color.brand : .white.opacity(0.14),
                        style: StrokeStyle(lineWidth: model.dropping ? 2 : 1.2, dash: [7, 5])
                    )
            )
            .frame(maxWidth: 430, maxHeight: 210)
            .overlay { content }
            .contentShape(Rectangle())
            .onTapGesture {
                guard !model.uploading else { return }
                Task { await model.pickAndUpload() }
            }
            .onDrop(of: [.fileURL], isTargeted: $model.dropping) { providers in
                Task { await model.upload(await urls(from: providers)) }
                return true
            }
            .animation(.easeOut(duration: 0.15), value: model.dropping)
    }

    private var content: some View {
        VStack(spacing: 10) {
            if model.uploading {
                ProgressView().controlSize(.large)
                Text(model.uploadProgress)
                    .font(.callout).foregroundStyle(.secondary)
            } else {
                Image(systemName: "arrow.up.doc")
                    .font(.system(size: 30, weight: .light))
                    .foregroundStyle(Color.brand)
                    .frame(width: 66, height: 66)
                    .background(Circle().fill(Color.brand.opacity(0.12)))
                Text("把图片拖到这里").font(.title3.weight(.medium))
                Text("或点击选择文件").font(.caption).foregroundStyle(.secondary)
            }
        }
    }

    private var formatRow: some View {
        VStack(spacing: 7) {
            Text("上传后复制").font(.caption).foregroundStyle(.secondary)
            PillRow(items: LinkFormat.allCases, label: \.label, selection: $model.linkFormat)
        }
    }

    @ViewBuilder
    private var limits: some View {
        if let t = model.quota?.tier {
            HStack(spacing: 16) {
                if t.dailyUploadCount > 0, let used = model.quota?.uploadsToday {
                    let left = max(0, t.dailyUploadCount - used)
                    // Shown before the upload, not after the failure: unlike the
                    // per-minute limit, the daily count does not clear until
                    // tomorrow, so it is not something to retry through.
                    stat("今日剩余", "\(left) 张", warn: left <= 5)
                }
                stat("单文件上限", model.bytes(t.maxFileSize))
                stat("支持格式", t.allowedFormats.prefix(4).joined(separator: " · ").uppercased())
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 11)
            .panelSurface(12)
        }
    }

    private func stat(_ k: String, _ v: String, warn: Bool = false) -> some View {
        VStack(spacing: 2) {
            Text(k).font(.caption2).foregroundStyle(.tertiary)
            Text(v).font(.callout.weight(.medium))
                .foregroundStyle(warn ? .orange : .primary)
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
