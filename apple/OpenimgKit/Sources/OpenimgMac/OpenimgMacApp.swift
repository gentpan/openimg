import SwiftUI
import AppKit
import OpenimgKit

@main
struct OpenimgMacApp: App {
    @StateObject private var model = AppModel.shared

    var body: some Scene {
        Window("Openimg", id: "main") {
            RootView(model: model)
        }
        .defaultSize(width: 980, height: 660)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("上传图片…") {
                    model.tab = .upload
                    Task { await model.pickAndUpload() }
                }
                .keyboardShortcut("u")
            }
            CommandGroup(after: .toolbar) {
                Button("刷新图库") { Task { await model.load() } }
                    .keyboardShortcut("r")
            }
        }
    }
}

struct RootView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()

            switch model.tab {
            case .gallery:  GalleryView(model: model)
            case .upload:   UploadView(model: model)
            case .settings: SettingsView(model: model)
            }

            if !model.status.isEmpty {
                Divider()
                HStack(spacing: 6) {
                    Text(model.status).font(.caption).lineLimit(1)
                    Spacer()
                    Button {
                        model.status = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(.tertiary)
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 6)
                .background(.quaternary.opacity(0.4))
            }
        }
        .frame(minWidth: 820, minHeight: 520)
        .task { await model.restore() }
    }

    private var header: some View {
        HStack(spacing: 4) {
            ForEach(Tab.allCases) { t in
                Button {
                    model.tab = t
                } label: {
                    Label(t.label, systemImage: t.icon)
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background(
                            RoundedRectangle(cornerRadius: 6)
                                .fill(model.tab == t ? Color.accentColor.opacity(0.15) : .clear)
                        )
                        .foregroundStyle(model.tab == t ? Color.accentColor : .primary)
                }
                .buttonStyle(.plain)
                // Gallery and upload need a connection; settings is where you
                // go to get one, so it stays reachable.
                .disabled(!model.connected && t != .settings)
            }

            Spacer()

            if let a = model.account {
                if let q = model.quota {
                    Text("\(model.bytes(q.availableBytes)) 可用")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Text(a.name.isEmpty ? a.email : a.name)
                    .font(.callout)
            } else {
                Text("未连接").font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 8)
    }
}
