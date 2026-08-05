import SwiftUI
import AppKit
import OpenimgKit

struct SettingsView: View {
    @ObservedObject var model: AppModel
    /// nil means "showing whatever the server says". It only holds a value
    /// while the field is being edited, so the card never has to be told that
    /// the account changed underneath it.
    @State private var draftName: String?
    @State private var editingName = false
    @State private var code = ""
    @State private var newPassword = ""
    @FocusState private var nameFocused: Bool

    var body: some View {
        ScrollView {
            // Two columns, matching the overview page.
            //
            // Six cards in one 560pt column inside a 1240pt window made the
            // page twice as long as it needed to be while leaving a third of
            // the width empty on either side. Split by what the cards are
            // about: who you are and how you get in on the left, what happens
            // to your images and where they go on the right.
            HStack(alignment: .top, spacing: 16) {
                VStack(spacing: 16) {
                    profileCard
                    securityCard
                    dangerCard
                }
                VStack(spacing: 16) {
                    conversionCard
                    locationCard
                    siteCard
                }
            }
            .frame(maxWidth: 980)
            .frame(maxWidth: .infinity)   // centres the block in a wide window
            .padding(.horizontal, 22)
            .padding(.bottom, 22)
        }
        .frame(maxWidth: .infinity)
        .task(id: model.account?.id) { await model.loadStats() }
    }

    // MARK: - Cards

    /// Editable, where it used to be a read-only block.
    ///
    /// It was read-only because the nickname and avatar routes were in the
    /// cookie-only group and this client holds a token, so every write would
    /// have 401'd. They now sit alongside the other things a token may do to
    /// its own account — see the note in router.go for where that line is.
    private var profileCard: some View {
        SettingsCard("个人资料", "person.crop.circle") {
            if let a = model.account {
                HStack(alignment: .top, spacing: 14) {
                    avatarWell(a)

                    VStack(alignment: .leading, spacing: 8) {
                        nameField(a)
                        Text(a.email).font(.callout).foregroundStyle(.secondary)
                        HStack(spacing: 6) {
                            tag(a.role)
                            // Role and tier are separate fields that happen to
                            // carry the same word for admins; printing both
                            // gives "admin admin".
                            if let t = model.quota?.tier, t.name != a.role { tag(t.name) }
                        }
                    }
                    Spacer(minLength: 0)
                }
            }
        }
    }

    /// The picture doubles as its own button: hovering reveals the actions over
    /// it, so the card does not carry two buttons for something most people
    /// set once.
    private func avatarWell(_ a: Account) -> some View {
        VStack(spacing: 6) {
            Avatar(account: a, size: 62, client: try? model.client())
                .overlay(
                    Circle().strokeBorder(.white.opacity(0.12), lineWidth: 1)
                )
            HStack(spacing: 4) {
                Button("更换") { Task { await model.pickAvatar() } }
                    .buttonStyle(LinkButton()).font(.caption2)
                if a.avatarURL?.isEmpty == false {
                    Text("·").font(.caption2).foregroundStyle(.quaternary)
                    Button("移除") { Task { await model.removeAvatar() } }
                        .buttonStyle(LinkButton()).font(.caption2)
                }
            }
            .disabled(model.busy)
        }
    }

    /// Commits on Return and on losing focus, and reverts on Escape.
    ///
    /// A save button for one short string is a button people forget to press;
    /// blur-to-save is what the website does, so the two agree.
    private func nameField(_ a: Account) -> some View {
        TextField("昵称", text: Binding(
            get: { draftName ?? a.name },
            set: { draftName = $0 }
        ))
            .textFieldStyle(.plain)
            .font(.title3.weight(.medium))
            .padding(.horizontal, 8).padding(.vertical, 5)
            .background(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(.white.opacity(editingName ? 0.08 : 0))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .strokeBorder(.white.opacity(editingName ? 0.14 : 0), lineWidth: 1)
            )
            .frame(maxWidth: 260)
            .onSubmit { commitName() }
            .onExitCommand { draftName = nil; editingName = false }
            .focused($nameFocused)
            .onChange(of: nameFocused) { _, focused in
                editingName = focused
                if !focused { commitName() }
            }
            .animation(.easeOut(duration: 0.12), value: editingName)
    }

    private func commitName() {
        guard let draft = draftName else { return }
        // Dropped before the request, not after: `saveNickname` refreshes the
        // account, and whatever the server decided to store is then what shows
        // — including a trim or a 32-character truncation the user did not do.
        draftName = nil
        Task { await model.saveNickname(draft) }
    }

    /// Read-only on purpose.
    ///
    /// Creating and editing a storage profile means handing over the S3 access
    /// key and secret, and those routes are cookie-only by design — a token
    /// pasted into a PicGo config must not be able to read them back. So the
    /// app shows where the bytes actually sit and sends the user to the site to
    /// change it. The numbers come from /api/storage/summary, which the token
    /// can already reach.
    private var locationCard: some View {
        SettingsCard("存储位置", "externaldrive.connected.to.line.below") {
            if let profiles = model.summary?.byProfile, !profiles.isEmpty {
                VStack(spacing: 0) {
                    ForEach(Array(profiles.enumerated()), id: \.element.id) { i, p in
                        if i > 0 { Divider().overlay(Color.white.opacity(0.06)) }
                        HStack(spacing: 10) {
                            Image(systemName: p.kind == "platform"
                                  ? "cube.box" : "externaldrive.badge.person.crop")
                                .font(.system(size: 14))
                                .foregroundStyle(Color.brand)
                                .frame(width: 20)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(p.name).font(.callout)
                                // The platform pool's name already is its kind;
                                // printing both gives "平台存储池 平台存储池".
                                if let k = kindLabel(p.kind), k != p.name {
                                    Text(k).font(.caption2).foregroundStyle(.tertiary)
                                }
                            }
                            Spacer()
                            VStack(alignment: .trailing, spacing: 1) {
                                Text(model.bytes(p.bytes)).font(.callout.monospacedDigit())
                                Text("\(p.images) 张")
                                    .font(.caption2).foregroundStyle(.tertiary)
                            }
                        }
                        .padding(.vertical, 9)
                    }
                }
                Text("新增或修改存储位置需要填写密钥，见下方「在网站上管理」")
                    .font(.caption2).foregroundStyle(.tertiary)
                    .padding(.top, 8)
            } else if model.statsLoading {
                ProgressView().controlSize(.small).frame(maxWidth: .infinity)
            } else {
                Text("还没有图片，看不出存的位置")
                    .font(.callout).foregroundStyle(.secondary)
            }
        }
    }

    private func kindLabel(_ kind: String) -> String? {
        switch kind {
        case "platform": "平台存储池"
        case "s3", "r2": kind.uppercased() + " · 自有存储桶"
        default: kind.isEmpty ? nil : kind
        }
    }

    private func siteHint(_ text: String) -> some View {
        HStack(spacing: 6) {
            Text(text).font(.caption2).foregroundStyle(.tertiary)
            Button("去网站") {
                if let u = URL(string: model.server + "/settings") {
                    NSWorkspace.shared.open(u)
                }
            }
            .buttonStyle(LinkButton()).font(.caption2)
            Spacer(minLength: 0)
        }
        .padding(.top, 8)
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
                setting("最大宽度",
                        model.uploadMode == .original
                        ? "保留原图时不缩放" : "超过就等比缩小，只影响之后上传的图片") {
                    PillRow(items: allowedMaxWidths.map(UploadView.Width.init),
                            label: \.label, selection: widthPref)
                        // Original mode ships the bytes untouched, so a width
                        // here does nothing. The web disables it; leaving it
                        // live on this end just invites setting a value that
                        // silently has no effect.
                        .disabled(model.uploadMode == .original)
                        .opacity(model.uploadMode == .original ? 0.45 : 1)
                }
                if let t = model.quota?.tier {
                    // The limits these three controls operate inside. They were
                    // a card of their own, which meant a read-only card sitting
                    // in a page of settings; against the switches they bound,
                    // they read as the constraint they are.
                    VStack(spacing: 0) {
                        Divider().overlay(Color.white.opacity(0.06))
                        row("单文件上限", model.bytes(t.maxFileSize))
                        Divider().overlay(Color.white.opacity(0.06))
                        row("每日上传", t.dailyUploadCount > 0 ? "\(t.dailyUploadCount) 张" : "不限")
                        Divider().overlay(Color.white.opacity(0.06))
                        row("支持格式", t.allowedFormats.joined(separator: " · ").uppercased())
                    }
                    .padding(.top, 2)
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

    /// Password, passkeys and linked providers — the same set the website
    /// offers, minus linking.
    ///
    /// These used to be website-only on the grounds that a token must not reach
    /// account management. Checking what the handlers actually require showed
    /// that reasoning was aimed at the wrong thing: changing a password and
    /// enrolling a passkey are both gated on a code mailed to the account's own
    /// address, so the second factor was never the cookie. See router.go.
    ///
    /// Linking a provider is still missing, and for a real reason rather than a
    /// policy one: it is a full-page redirect carrying an intent cookie, and
    /// this app's web session is ephemeral, so the callback would have nothing
    /// to attach the link to.
    private var securityCard: some View {
        SettingsCard("登录与安全", "lock.shield") {
            VStack(alignment: .leading, spacing: 16) {
                passwordSection
                Divider().overlay(Color.white.opacity(0.06))
                passkeySection
            }
        }
        .task(id: model.account?.id) { await model.loadPasskeys() }
    }

    @ViewBuilder
    private var passwordSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                Text("修改密码").font(.callout)
                Spacer()
                Text(model.codeSent ? "验证码 10 分钟内有效" : "验证码会发到你的邮箱")
                    .font(.caption2).foregroundStyle(.tertiary)
            }
            if model.codeSent {
                HStack(spacing: 8) {
                    Field(icon: "number") {
                        TextField("6 位验证码", text: $code)
                    }
                    .frame(width: 150)
                    Field(icon: "lock") {
                        SecureField("新密码（至少 8 位）", text: $newPassword)
                    }
                    Button("确认") {
                        Task {
                            if await model.changePassword(code: code, newPassword: newPassword) {
                                code = ""; newPassword = ""
                            }
                        }
                    }
                    .buttonStyle(BrandButton())
                    .disabled(code.count != 6 || newPassword.count < 8 || model.busy)
                }
                Button("取消") { model.codeSent = false; code = ""; newPassword = "" }
                    .buttonStyle(LinkButton()).font(.caption2)
            } else {
                Button("发送验证码") { Task { await model.sendCode(.password) } }
                    .buttonStyle(QuietButton())
                    .disabled(model.busy)
            }
        }
    }

    @ViewBuilder
    private var passkeySection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                Text("Passkey").font(.callout)
                Spacer()
                Text("免密码登录，用触控 ID 或手机确认")
                    .font(.caption2).foregroundStyle(.tertiary)
            }
            if model.passkeys.isEmpty {
                Text("还没有添加 Passkey").font(.caption).foregroundStyle(.secondary)
            } else {
                VStack(spacing: 0) {
                    ForEach(model.passkeys) { p in
                        if p.id != model.passkeys.first?.id {
                            Divider().overlay(Color.white.opacity(0.06))
                        }
                        HStack(spacing: 10) {
                            Image(systemName: "person.badge.key")
                                .font(.system(size: 13))
                                .foregroundStyle(Color.brand)
                                .frame(width: 18)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(p.name).font(.callout)
                                if let d = p.lastUsedAt {
                                    Text("最近使用 " + d.formatted(date: .abbreviated, time: .omitted))
                                        .font(.caption2).foregroundStyle(.tertiary)
                                } else if let d = p.createdAt {
                                    Text("添加于 " + d.formatted(date: .abbreviated, time: .omitted))
                                        .font(.caption2).foregroundStyle(.tertiary)
                                }
                            }
                            Spacer()
                            Button("删除") { Task { await model.deletePasskey(p) } }
                                .buttonStyle(LinkButton()).font(.caption2)
                        }
                        .padding(.vertical, 8)
                    }
                }
            }
            // Enrolling needs the WebAuthn ceremony, which needs the
            // associated-domains entitlement this build cannot carry.
            Text("添加 Passkey 需要在网站上完成")
                .font(.caption2).foregroundStyle(.tertiary)
        }
    }

    /// Everything this app cannot do, in one place.
    ///
    /// These are the cookie-only routes — a token must not be able to change a
    /// password, mint more tokens, or read storage credentials, and that is a
    /// deliberate boundary rather than a gap. But it was previously expressed
    /// as one grey sentence in the corner of another card, which is the same as
    /// not saying it: a Mac user is exactly the person who needs an API token,
    /// and they had no way to find out where to get one.
    private var siteCard: some View {
        SettingsCard("在网站上管理", "safari") {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Self.siteLinks, id: \.0) { title, detail, path in
                    if title != Self.siteLinks.first?.0 {
                        Divider().overlay(Color.white.opacity(0.06))
                    }
                    Button {
                        if let u = URL(string: model.server + path) {
                            NSWorkspace.shared.open(u)
                        }
                    } label: {
                        HStack(spacing: 10) {
                            VStack(alignment: .leading, spacing: 1) {
                                Text(title).font(.callout).foregroundStyle(.primary)
                                Text(detail).font(.caption2).foregroundStyle(.tertiary)
                            }
                            Spacer(minLength: 8)
                            Image(systemName: "arrow.up.right")
                                .font(.system(size: 10, weight: .semibold))
                                .foregroundStyle(.tertiary)
                        }
                        .padding(.vertical, 9)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                }
            }
        }
    }

    private static let siteLinks: [(String, String, String)] = [
        ("API Token", "给 PicGo、Typora、curl 等工具上传用", "/settings"),
        ("绑定 Google / GitHub", "绑定要整页跳转，原生端做不了", "/settings"),
        ("添加 Passkey", "注册需要在网页里完成，删除可以在本页做", "/settings"),
        ("删除账号", "", "/settings"),
    ]

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
