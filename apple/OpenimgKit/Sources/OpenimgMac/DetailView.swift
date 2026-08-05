import SwiftUI
import AppKit
import OpenimgKit

struct DetailView: View {
    @ObservedObject var model: AppModel
    let img: RemoteImage
    @State private var copied: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("图片详情").font(.headline)
                    Spacer()
                    Button {
                        model.detail = nil
                    } label: {
                        Image(systemName: "xmark")
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(.secondary)
                }

                // The thumbnail, not the original. This panel is ~280pt wide;
                // one of the images on the live site is a 4.2 MB PNG whose
                // 6 KB thumbnail renders identically at this size.
                Thumbnail(url: img.thumbURL, client: try? model.client())
                    .frame(height: 150)
                    .clipShape(RoundedRectangle(cornerRadius: 8))

                Text(img.origName)
                    .font(.callout)
                    .textSelection(.enabled)

                Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 5) {
                    row("尺寸", "\(img.width) × \(img.height)")
                    row("占用", model.bytes(img.sizeStored))
                    row("格式", img.ext.uppercased())
                    row("上传", img.createdAt.formatted(date: .numeric, time: .shortened))
                }
                .font(.caption)

                Divider()

                Text("链接").font(.caption).foregroundStyle(.secondary)
                ForEach(LinkFormat.allCases, id: \.self) { f in
                    Button {
                        model.copy(f.render(img))
                        copied = f.label
                    } label: {
                        HStack {
                            Text(f.label).frame(width: 68, alignment: .leading)
                            Text(f.render(img))
                                .lineLimit(1)
                                .truncationMode(.middle)
                                .foregroundStyle(.secondary)
                            Spacer(minLength: 0)
                            Image(systemName: copied == f.label ? "checkmark" : "doc.on.doc")
                                .foregroundStyle(copied == f.label ? Color.green : .secondary)
                        }
                        .font(.caption)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                }

                if let short = img.shortURL {
                    Divider()
                    Button {
                        if let u = URL(string: short) { NSWorkspace.shared.open(u) }
                    } label: {
                        Label("在浏览器打开分享页", systemImage: "safari")
                    }
                    .font(.caption)
                }

                Divider()
                Button(role: .destructive) {
                    Task { await model.delete(img) }
                } label: {
                    Label("删除这张图片", systemImage: "trash")
                }
                .font(.caption)
            }
            .padding(14)
        }
        .onChange(of: img.id) { copied = nil }
    }

    private func row(_ k: String, _ v: String) -> some View {
        GridRow {
            Text(k).foregroundStyle(.secondary)
            Text(v).textSelection(.enabled)
        }
    }
}
