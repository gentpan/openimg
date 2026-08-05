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
        VStack(alignment: .leading, spacing: 10) {
            Label(model.connected ? "已登录" : "登录", systemImage: "person.badge.key")
                .font(.headline)

            if model.connected {
                HStack {
                    Text("这台设备的凭证保存在钥匙串里，重开应用会自动登录。")
                        .font(.caption).foregroundStyle(.secondary)
                    Spacer()
                    Button("退出登录", role: .destructive) { model.signOut() }
                }
            } else {
                Picker("", selection: $model.useToken) {
                    Text("密码登录").tag(false)
                    Text("访问令牌").tag(true)
                }
                .pickerStyle(.segmented)
                .labelsHidden()

                if model.useToken {
                    SecureField("oimg_…", text: $model.token)
                        .textFieldStyle(.roundedBorder)
                        .onSubmit { Task { await model.connect() } }
                    hint("在网站的「账号设置 → API Token」里生成。适合不想把账号密码交给第三方程序的情况。")
                    Button(model.busy ? "连接中…" : "连接") { Task { await model.connect() } }
                        .buttonStyle(.borderedProminent)
                        .disabled(model.busy || model.token.isEmpty)
                        .keyboardShortcut(.defaultAction)
                } else {
                    TextField("邮箱", text: $model.email)
                        .textFieldStyle(.roundedBorder)
                    SecureField("密码", text: $model.password)
                        .textFieldStyle(.roundedBorder)
                        .onSubmit { Task { await model.signIn() } }
                    // Says what the password is actually used for. "We store a
                    // token, not your password" is only reassuring if it is
                    // stated before the field is filled in, not after.
                    hint("密码只用来换取一枚这台设备专用的访问令牌，不会被保存。令牌存在钥匙串，之后每次打开都自动登录。")
                    Button(model.busy ? "登录中…" : "登录") { Task { await model.signIn() } }
                        .buttonStyle(.borderedProminent)
                        .disabled(model.busy || model.email.isEmpty || model.password.isEmpty)
                        .keyboardShortcut(.defaultAction)
                }
            }

            HStack {
                Spacer()
                Button("在网站上打开") {
                    if let u = URL(string: model.server) { NSWorkspace.shared.open(u) }
                }
                .help("用默认浏览器打开 Openimg")
                .buttonStyle(.link)
                .font(.caption)
            }
        }
    }

    private func hint(_ t: String) -> some View {
        Text(t)
            .font(.caption)
            .foregroundStyle(.secondary)
            .fixedSize(horizontal: false, vertical: true)
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
