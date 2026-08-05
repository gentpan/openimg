import SwiftUI
import OpenimgKit
import UniformTypeIdentifiers

struct UploadView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 16) {
            Spacer()

            RoundedRectangle(cornerRadius: 12)
                .strokeBorder(style: StrokeStyle(lineWidth: 1.5, dash: [6]))
                .foregroundStyle(model.dropping ? Color.accentColor : Color.secondary.opacity(0.4))
                .frame(maxWidth: 420, maxHeight: 200)
                .overlay {
                    VStack(spacing: 6) {
                        if model.uploading {
                            ProgressView()
                            Text(model.uploadProgress).font(.caption).foregroundStyle(.secondary)
                        } else {
                            Image(systemName: "arrow.up.doc")
                                .font(.system(size: 30))
                                .foregroundStyle(.tertiary)
                            Text("把图片拖到这里").font(.title3)
                            Text("或点击选择文件").font(.caption).foregroundStyle(.secondary)
                        }
                    }
                }
                .contentShape(Rectangle())
                .onTapGesture {
                    guard !model.uploading else { return }
                    Task { await model.pickAndUpload() }
                }
                .onDrop(of: [.fileURL], isTargeted: $model.dropping) { providers in
                    Task { await model.upload(await urls(from: providers)) }
                    return true
                }

            HStack(spacing: 8) {
                Text("上传后复制").font(.caption).foregroundStyle(.secondary)
                Picker("", selection: $model.linkFormat) {
                    ForEach(LinkFormat.allCases, id: \.self) { Text($0.label).tag($0) }
                }
                .labelsHidden()
                .pickerStyle(.segmented)
                .frame(width: 300)
            }

            // Shown before the upload rather than after the failure: unlike the
            // per-minute rate limit, the daily count does not clear until
            // tomorrow, so hitting it is not something to retry through.
            if let t = model.quota?.tier {
                VStack(spacing: 3) {
                    if t.dailyUploadCount > 0, let used = model.quota?.uploadsToday {
                        Text("今日还可上传 \(max(0, t.dailyUploadCount - used)) 张")
                    }
                    Text("单文件上限 \(model.bytes(t.maxFileSize))　支持 \(t.allowedFormats.joined(separator: " / "))")
                }
                .font(.caption2)
                .foregroundStyle(.secondary)
            }

            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(24)
    }

    /// NSItemProvider hands back a file URL as its `Data` representation, not
    /// as a URL — decoding it any other way silently yields nil for every drop.
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
