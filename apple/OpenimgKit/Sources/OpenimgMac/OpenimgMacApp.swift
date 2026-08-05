import SwiftUI
import AppKit
import OpenimgKit

@main
struct OpenimgMacApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        MenuBarExtra("Openimg", systemImage: "photo.on.rectangle.angled") {
            MenuView(model: model)
        }
        .menuBarExtraStyle(.window) // .menu cannot host a drop target or a text field
    }
}

struct MenuView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if model.connected {
                connected
            } else {
                setup
            }

            if !model.status.isEmpty {
                Text(model.status)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                    .fixedSize(horizontal: false, vertical: true)
            }

            Divider()
            HStack {
                if model.connected {
                    Button("断开", action: model.disconnect)
                        .buttonStyle(.link)
                }
                Spacer()
                Button("退出") { NSApp.terminate(nil) }
                    .buttonStyle(.link)
            }
            .font(.caption)
        }
        .padding(14)
        .frame(width: 320)
    }

    // MARK: - Not connected

    private var setup: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("连接到图床").font(.headline)

            TextField("服务器地址", text: $model.server)
                .textFieldStyle(.roundedBorder)
            SecureField("访问令牌 oimg_…", text: $model.token)
                .textFieldStyle(.roundedBorder)

            Text("在网站的「账号设置 → API Token」里生成。令牌保存在钥匙串，不写入配置文件。")
                .font(.caption2)
                .foregroundStyle(.secondary)

            Button(model.busy ? "连接中…" : "连接") {
                Task { await model.connect() }
            }
            .disabled(model.busy || model.token.isEmpty)
            .keyboardShortcut(.defaultAction)
        }
    }

    // MARK: - Connected

    private var connected: some View {
        VStack(alignment: .leading, spacing: 10) {
            if let a = model.account {
                HStack {
                    Text(a.name.isEmpty ? a.email : a.name).font(.headline)
                    Spacer()
                    if let q = model.quota {
                        Text("\(model.bytes(q.availableBytes)) 可用")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
            }

            // The daily count is the limit a screenshot workflow actually hits,
            // and unlike the rate limit it does not clear until tomorrow — so
            // it is shown before the upload rather than after the failure.
            if let q = model.quota, q.tier.dailyUploadCount > 0 {
                let left = max(0, q.tier.dailyUploadCount - q.uploadsToday)
                Text("今日还可上传 \(left) 张")
                    .font(.caption2)
                    .foregroundStyle(left <= 5 ? .orange : .secondary)
            }

            dropZone

            Picker("链接格式", selection: $model.format) {
                ForEach(LinkFormat.allCases, id: \.self) { Text($0.label).tag($0) }
            }
            .pickerStyle(.segmented)
            .labelsHidden()

            if !model.recent.isEmpty {
                Text("最近上传").font(.caption).foregroundStyle(.secondary)
                ScrollView {
                    VStack(alignment: .leading, spacing: 2) {
                        ForEach(model.recent) { img in
                            Button {
                                model.copy(model.format.render(img))
                                model.status = "已复制 \(img.origName)"
                            } label: {
                                HStack {
                                    Text(img.origName).lineLimit(1)
                                    Spacer()
                                    Text(model.bytes(img.sizeStored))
                                        .foregroundStyle(.tertiary)
                                }
                                .font(.caption)
                                .contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
                .frame(maxHeight: 140)
            }
        }
    }

    private var dropZone: some View {
        RoundedRectangle(cornerRadius: 8)
            .strokeBorder(style: StrokeStyle(lineWidth: 1, dash: [4]))
            .foregroundStyle(model.dropping ? Color.accentColor : Color.secondary.opacity(0.5))
            .frame(height: 62)
            .overlay {
                VStack(spacing: 2) {
                    Text(model.busy ? "上传中…" : "把图片拖到这里")
                    Text("或点击选择文件").font(.caption2).foregroundStyle(.secondary)
                }
                .font(.callout)
            }
            .contentShape(Rectangle())
            .onTapGesture { Task { await model.pickAndUpload() } }
            .onDrop(of: [.fileURL], isTargeted: $model.dropping) { providers in
                Task {
                    var urls: [URL] = []
                    for p in providers {
                        if let url = try? await p.loadItem(forTypeIdentifier: "public.file-url") as? Data,
                           let u = URL(dataRepresentation: url, relativeTo: nil) {
                            urls.append(u)
                        }
                    }
                    await model.upload(urls)
                }
                return true
            }
    }
}
