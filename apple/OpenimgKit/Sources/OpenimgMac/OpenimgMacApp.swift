import SwiftUI
import AppKit
import OpenimgKit

@main
struct OpenimgMacApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate

    var body: some Scene {
        MenuBarExtra("Openimg", systemImage: "photo.on.rectangle.angled") {
            MenuView(model: AppModel.shared)
        }
        .menuBarExtraStyle(.window) // .menu cannot host a drop target or a text field
    }
}

/// Opens a real window on launch, and again whenever the app is re-opened.
///
/// A status item alone is not a findable UI. On a notched MacBook the menu bar
/// silently drops whatever does not fit behind the notch — no overflow arrow,
/// no indication that anything is missing — and with a second display attached
/// the item may not even be on the screen being looked at. The app is running
/// correctly and the user has no way to reach it, which is indistinguishable
/// from it being broken.
///
/// So the window is the primary surface and the menu bar item is the shortcut,
/// not the other way around. Re-running the app brings the window back rather
/// than starting a second copy, which is also what double-clicking it in Finder
/// should do.
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var window: NSWindow?

    func applicationDidFinishLaunching(_ notification: Notification) {
        showWindow()
    }

    /// Fires when the app is launched while already running, or its Dock icon
    /// is clicked. Without this a second `open` would appear to do nothing.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        showWindow()
        return true
    }

    @MainActor
    private func showWindow() {
        if let w = window {
            w.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }
        let host = NSHostingController(rootView: MenuView(model: AppModel.shared))
        let w = NSWindow(contentViewController: host)
        w.title = "Openimg"
        w.styleMask = [.titled, .closable, .miniaturizable]
        w.isReleasedWhenClosed = false // reopened later, so it must survive a close
        w.center()
        w.makeKeyAndOrderFront(nil)
        // LSUIElement apps do not come forward on their own.
        NSApp.activate(ignoringOtherApps: true)
        window = w
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
