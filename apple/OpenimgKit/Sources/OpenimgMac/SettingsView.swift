import SwiftUI
import AppKit
import OpenimgKit

struct SettingsView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                accountCard
                storageCard
                conversionCard
                tierCard
                dangerCard
            }
            .frame(maxWidth: 560, alignment: .leading)
            .padding(.horizontal, 22)
            .padding(.bottom, 22)
        }
        .frame(maxWidth: .infinity)
    }

    // MARK: - Cards

    private var accountCard: some View {
        SettingsCard("账号", "person.crop.circle") {
            if let a = model.account {
                HStack(spacing: 14) {
                    Avatar(account: a, size: 54, client: try? model.client())
                    VStack(alignment: .leading, spacing: 3) {
                        Text(a.name.isEmpty ? "（未设置昵称）" : a.name)
                            .font(.title3.weight(.medium))
                        Text(a.email).font(.callout).foregroundStyle(.secondary)
                        HStack(spacing: 6) {
                            tag(a.role)
                            // Role and tier are separate fields that happen to
                            // carry the same word for admins; printing both
                            // gives "admin admin".
                            if let t = model.quota?.tier, t.name != a.role { tag(t.name) }
                        }
                        .padding(.top, 2)
                    }
                    Spacer()
                }
            }
        }
    }

    private var storageCard: some View {
        SettingsCard("空间", "internaldrive") {
            if let q = model.quota {
                VStack(alignment: .leading, spacing: 10) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(model.bytes(q.availableBytes))
                            .font(.system(size: 26, weight: .semibold, design: .rounded))
                            .foregroundStyle(Color.brand)
                        Text("可用").foregroundStyle(.secondary)
                        Spacer()
                        Text("\(q.imageCount) 张").font(.callout).foregroundStyle(.secondary)
                    }
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            Capsule().fill(.white.opacity(0.10))
                            Capsule().fill(Color.brand)
                                .frame(width: max(4, geo.size.width * min(1,
                                    q.quotaBytes > 0 ? Double(q.usedBytes) / Double(q.quotaBytes) : 0)))
                        }
                    }
                    .frame(height: 6)
                    HStack {
                        Text("已用 \(model.bytes(q.usedBytes))")
                        Spacer()
                        Text("总量 \(model.bytes(q.quotaBytes))")
                    }
                    .font(.caption).foregroundStyle(.secondary)
                }
            }
        }
    }

    /// The same three values the upload page offers. Both write to the account,
    /// so whichever one the user reaches for, the website agrees.
    private var conversionCard: some View {
        SettingsCard("图片处理", "wand.and.stars") {
            VStack(alignment: .leading, spacing: 14) {
                setting("处理方式", model.uploadMode.detail) {
                    PillRow(items: UploadMode.allCases, label: \.label, selection: pref($model.uploadMode))
                }
                setting("衍生格式", "多存一份现代格式，浏览器优先取它") {
                    PillRow(items: VariantFormat.allCases, label: \.label, selection: pref($model.variantFormat))
                }
                setting("最大宽度", "超过就等比缩小，只影响之后上传的图片") {
                    PillRow(items: allowedMaxWidths.map(UploadView.Width.init),
                            label: \.label, selection: widthPref)
                }
            }
        }
    }

    private func setting<C: View>(_ title: String, _ hint: String,
                                  @ViewBuilder control: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.callout)
                Spacer()
                Text(hint).font(.caption2).foregroundStyle(.tertiary)
            }
            control()
        }
    }

    private func pref<T>(_ b: Binding<T>) -> Binding<T> {
        Binding(get: { b.wrappedValue },
                set: { b.wrappedValue = $0; Task { await model.savePreferences() } })
    }

    private var widthPref: Binding<UploadView.Width> {
        Binding(get: { UploadView.Width(model.maxImageWidth) },
                set: { model.maxImageWidth = $0.px; Task { await model.savePreferences() } })
    }

    private var tierCard: some View {
        SettingsCard("上传限制", "slider.horizontal.3") {
            if let t = model.quota?.tier {
                VStack(spacing: 0) {
                    row("单文件上限", model.bytes(t.maxFileSize))
                    Divider().overlay(Color.white.opacity(0.06))
                    row("每日上传", t.dailyUploadCount > 0 ? "\(t.dailyUploadCount) 张" : "不限")
                    Divider().overlay(Color.white.opacity(0.06))
                    row("支持格式", t.allowedFormats.joined(separator: " · ").uppercased())
                }
                // These are cookie-only on the server — a token deliberately
                // cannot reach account management — so they are a link out
                // rather than a control that would fail here.
                Text("绑定自有 R2 / S3、修改昵称与密码需要在网站上操作")
                    .font(.caption2).foregroundStyle(.tertiary)
                    .padding(.top, 8)
            }
        }
    }

    private var dangerCard: some View {
        SettingsCard("这台设备", "laptopcomputer") {
            HStack(alignment: .top) {
                Text("凭证保存在钥匙串里，重开应用会自动登录。退出登录只会从这台\n设备移除它，服务器上的令牌需要在网站里删除。")
                    .font(.caption).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 12)
                Button("退出登录") { model.signOut() }
                    .buttonStyle(DangerButton())
            }
            HStack {
                Spacer()
                Button("在网站上打开") {
                    if let u = URL(string: model.server) { NSWorkspace.shared.open(u) }
                }
                .buttonStyle(QuietButton())
            }
            .padding(.top, 4)
        }
    }

    // MARK: - Bits

    private func row(_ k: String, _ v: String) -> some View {
        HStack {
            Text(k).foregroundStyle(.secondary)
            Spacer()
            Text(v).multilineTextAlignment(.trailing)
        }
        .font(.callout)
        .padding(.vertical, 8)
    }

    private func tag(_ s: String) -> some View {
        Text(s)
            .font(.caption2.weight(.medium))
            .foregroundStyle(Color.brand)
            .padding(.horizontal, 7).padding(.vertical, 2)
            .background(Capsule().fill(Color.brand.opacity(0.16)))
    }
}

/// One card shape for the whole settings page, so sections stop each inventing
/// their own heading weight and padding.
struct SettingsCard<Content: View>: View {
    let title: String
    let icon: String
    @ViewBuilder let content: Content

    init(_ title: String, _ icon: String, @ViewBuilder content: () -> Content) {
        self.title = title; self.icon = icon; self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label(title, systemImage: icon)
                .font(.subheadline.weight(.medium))
                .foregroundStyle(.secondary)
            content
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .panelSurface()
    }
}
