import SwiftUI
import AppKit
import OpenimgKit

struct SettingsView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                connection
                if model.connected {
                    Divider()
                    accountInfo
                    Divider()
                    storage
                }
            }
            .frame(maxWidth: 520, alignment: .leading)
            .padding(24)
        }
        .frame(maxWidth: .infinity)
    }

    private var connection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Label("连接", systemImage: "link").font(.headline)

            TextField("服务器地址", text: $model.server)
                .textFieldStyle(.roundedBorder)
            SecureField("访问令牌 oimg_…", text: $model.token)
                .textFieldStyle(.roundedBorder)

            Text("在网站的「账号设置 → API Token」里生成。令牌存在钥匙串，按服务器分别保存，连自建实例不会覆盖公网那条。")
                .font(.caption)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            HStack {
                Button(model.busy ? "连接中…" : (model.connected ? "重新连接" : "连接")) {
                    Task { await model.connect() }
                }
                .disabled(model.busy || model.token.isEmpty)
                .keyboardShortcut(.defaultAction)

                if model.connected {
                    Button("断开", role: .destructive) { model.disconnect() }
                }
                Spacer()
                Button("打开网站") {
                    if let u = URL(string: model.server) { NSWorkspace.shared.open(u) }
                }
            }
        }
    }

    private var accountInfo: some View {
        VStack(alignment: .leading, spacing: 6) {
            Label("账号", systemImage: "person.crop.circle").font(.headline)
            if let a = model.account {
                Grid(alignment: .leading, horizontalSpacing: 14, verticalSpacing: 4) {
                    row("昵称", a.name.isEmpty ? "（未设置）" : a.name)
                    row("邮箱", a.email)
                    row("角色", a.role)
                    if let t = model.quota?.tier { row("用户组", t.name) }
                }
                .font(.callout)
            }
        }
    }

    private var storage: some View {
        VStack(alignment: .leading, spacing: 8) {
            Label("空间", systemImage: "internaldrive").font(.headline)
            if let q = model.quota {
                let used = q.quotaBytes > 0 ? Double(q.usedBytes) / Double(q.quotaBytes) : 0
                ProgressView(value: min(1, used))
                Text("\(model.bytes(q.usedBytes)) / \(model.bytes(q.quotaBytes))　剩余 \(model.bytes(q.availableBytes))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("\(q.imageCount) 张图片　今日已上传 \(q.uploadsToday) 张")
                    .font(.caption)
                    .foregroundStyle(.secondary)

                // Storage profiles and check-in are cookie-only on the server —
                // a token deliberately cannot reach account management — so
                // they are a link out rather than a broken control here.
                Text("绑定自有 R2 / S3、每日签到等需要在网站上操作。")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
        }
    }

    private func row(_ k: String, _ v: String) -> some View {
        GridRow {
            Text(k).foregroundStyle(.secondary)
            Text(v).textSelection(.enabled)
        }
    }
}
