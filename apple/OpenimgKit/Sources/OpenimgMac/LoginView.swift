import SwiftUI
import OpenimgKit

/// Sign-in as a standalone screen rather than a section inside settings.
///
/// It was a form buried under a heading, which put the one thing a new user
/// has to do at the same weight as everything they cannot do yet. A dedicated
/// screen also gets to say what the app is before asking for a password.
struct LoginView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ZStack {
            // A wash of brand colour behind the glass, so the card has
            // something to be translucent against on a plain desktop.
            RadialGradient(
                colors: [Color.brand.opacity(0.22), .clear],
                center: .top, startRadius: 40, endRadius: 520
            )
            .ignoresSafeArea()

            VStack(spacing: 0) {
                Spacer(minLength: 20)
                card
                Spacer(minLength: 20)
            }
            .frame(maxWidth: .infinity)
        }
    }

    private var card: some View {
        VStack(spacing: 18) {
            header

            VStack(spacing: 9) {
                if model.useToken {
                    field("访问令牌", systemImage: "key.fill") {
                        SecureField("oimg_…", text: $model.token)
                    }
                    Text("在网站的「账号设置 → API Token」里生成")
                        .font(.caption).foregroundStyle(.tertiary)
                        .frame(maxWidth: .infinity, alignment: .leading)
                } else {
                    field("邮箱", systemImage: "envelope.fill") {
                        TextField("you@example.com", text: $model.email)
                    }
                    field("密码", systemImage: "lock.fill") {
                        SecureField("", text: $model.password)
                    }
                }
            }

            primaryButton

            divider

            // Three equal squares. Icon-only because the marks carry the
            // meaning better than "使用 Google 继续" does at this size, and
            // three full-width rows would make the alternatives look like the
            // main path rather than the shortcut.
            HStack(spacing: 9) {
                ForEach(SignInMethod.allCases) { m in
                    squareButton(m)
                }
            }

            Button(model.useToken ? "改用邮箱密码登录" : "改用访问令牌登录") {
                withAnimation(.easeInOut(duration: 0.18)) { model.useToken.toggle() }
            }
            .buttonStyle(.plain)
            .font(.caption)
            .foregroundStyle(Color.brand)

            // Stated before the password field is filled in, not after. "We
            // store a token, not your password" is only reassuring in advance.
            Text(model.useToken
                 ? "令牌保存在钥匙串，不写入配置文件"
                 : "密码只用来换取一枚这台设备专用的令牌，不会被保存")
                .font(.caption2)
                .foregroundStyle(.tertiary)
                .multilineTextAlignment(.center)
        }
        .padding(28)
        .frame(width: 360)
        .glassCard(cornerRadius: 20)
        .shadow(color: .black.opacity(0.18), radius: 24, y: 10)
    }

    private var header: some View {
        VStack(spacing: 10) {
            Image(systemName: "photo.on.rectangle.angled")
                .font(.system(size: 30, weight: .light))
                .foregroundStyle(.white)
                .frame(width: 58, height: 58)
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Color.brand.gradient)
                )
                .shadow(color: Color.brand.opacity(0.35), radius: 10, y: 4)

            // "Openimg" in the brand face, the surrounding Chinese in the
            // system face — Ubuntu has no CJK coverage, and letting it fall
            // through mid-sentence gives two different-looking words.
            HStack(spacing: 0) {
                Text("登录 ").font(.title2.weight(.semibold))
                Text("Open").font(.brand(22, .bold))
                Text("img").font(.brand(22, .bold)).foregroundStyle(Color.brand)
            }
            Text("图片托管与分发").font(.caption).foregroundStyle(.secondary)
        }
        .padding(.bottom, 2)
    }

    private var divider: some View {
        HStack(spacing: 10) {
            Rectangle().fill(.quaternary).frame(height: 1)
            Text("或").font(.caption2).foregroundStyle(.tertiary)
            Rectangle().fill(.quaternary).frame(height: 1)
        }
    }

    private func squareButton(_ method: SignInMethod) -> some View {
        Button {
            Task { await model.signIn(with: method) }
        } label: {
            method.mark
                .frame(width: 20, height: 20)
                .frame(maxWidth: .infinity)
                .frame(height: 42)
                .contentShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        }
        .buttonStyle(OutlineButton())
        .disabled(model.busy)
        .help(method.title)
        .accessibilityLabel("使用 \(method.title) 登录")
    }

    private func field<C: View>(_ label: String, systemImage: String,
                                @ViewBuilder content: () -> C) -> some View {
        HStack(spacing: 9) {
            Image(systemName: systemImage)
                .font(.caption)
                .foregroundStyle(.tertiary)
                .frame(width: 15)
            content()
                .textFieldStyle(.plain)
                .font(.callout)
                .onSubmit { Task { await model.primaryAction() } }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .background(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .fill(.primary.opacity(0.05))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .strokeBorder(.primary.opacity(0.10), lineWidth: 0.8)
        )
        .accessibilityLabel(label)
    }

    private var primaryButton: some View {
        Button {
            Task { await model.primaryAction() }
        } label: {
            HStack(spacing: 7) {
                if model.busy { ProgressView().controlSize(.small).tint(.white) }
                Text(model.busy ? "登录中…" : "登录")
            }
            .font(.callout.weight(.medium))
            .frame(maxWidth: .infinity)
            .padding(.vertical, 9)
        }
        .buttonStyle(BrandButton())
        .disabled(model.busy || !model.canSubmit)
        .keyboardShortcut(.defaultAction)
    }
}

// MARK: - Button styles

private struct BrandButton: ButtonStyle {
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(.white)
            .background(
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(Color.brand.gradient)
                    .opacity(enabled ? (configuration.isPressed ? 0.82 : 1) : 0.4)
            )
            .animation(.easeOut(duration: 0.1), value: configuration.isPressed)
    }
}

private struct OutlineButton: ButtonStyle {
    @State private var hovering = false

    func makeBody(configuration: Configuration) -> some View {
        let shape = RoundedRectangle(cornerRadius: 10, style: .continuous)
        return configuration.label
            .background(shape.fill(.primary.opacity(configuration.isPressed ? 0.10 : hovering ? 0.06 : 0.02)))
            .overlay(shape.strokeBorder(.primary.opacity(0.12), lineWidth: 0.8))
            .onHover { hovering = $0 }
            .animation(.easeOut(duration: 0.12), value: hovering)
    }
}

// MARK: - Provider marks

enum SignInMethod: String, CaseIterable, Identifiable {
    case google, github, passkey
    var id: String { rawValue }

    var title: String {
        switch self {
        case .google: "Google"
        case .github: "GitHub"
        case .passkey: "Passkey"
        }
    }

    @ViewBuilder
    var mark: some View {
        switch self {
        case .google: GoogleMark()
        case .github: GitHubMark()
        case .passkey:
            // Touch ID is the shape people associate with a passkey on a Mac,
            // and it is a system symbol rather than a third-party brand, so it
            // carries no usage restrictions.
            Image(systemName: "touchid")
                .font(.system(size: 19, weight: .regular))
                .foregroundStyle(Color.brand)
        }
    }
}
