import SwiftUI
import OpenimgKit

/// Sign-in as its own screen rather than a section inside settings: it is the
/// one thing a new user must do, and burying it under a heading gave it the
/// same weight as everything they cannot do yet.
struct LoginView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ZStack {
            // A wash of brand colour so the card has something to be
            // translucent against rather than flat black.
            RadialGradient(colors: [Color.brand.opacity(0.28), .clear],
                           center: .init(x: 0.5, y: 0.15), startRadius: 30, endRadius: 560)
                .ignoresSafeArea()
            card
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var card: some View {
        VStack(spacing: 16) {
            header

            VStack(spacing: 8) {
                if model.useToken {
                    Field(icon: "key.fill") {
                        SecureField("oimg_…", text: $model.token)
                            .onSubmit { Task { await model.primaryAction() } }
                    }
                } else {
                    Field(icon: "envelope.fill") {
                        TextField("you@example.com", text: $model.email)
                    }
                    Field(icon: "lock.fill") {
                        SecureField("密码", text: $model.password)
                            .onSubmit { Task { await model.primaryAction() } }
                    }
                }
            }

            Button {
                Task { await model.primaryAction() }
            } label: {
                HStack(spacing: 7) {
                    if model.busy { ProgressView().controlSize(.small).tint(.white) }
                    Text(model.busy ? "登录中…" : "登录")
                }
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(BrandButton())
            .disabled(model.busy || !model.canSubmit)
            .keyboardShortcut(.defaultAction)

            divider

            // Three equal tiles. Icon-only because the marks carry the meaning
            // better than a label does at this size, and three full-width rows
            // would make the alternatives outrank the main path.
            HStack(spacing: 8) {
                ForEach(SignInMethod.allCases) { m in
                    Button { Task { await model.signIn(with: m) } } label: {
                        m.mark
                            .frame(width: 20, height: 20)
                            .frame(maxWidth: .infinity)
                            .frame(height: 40)
                    }
                    .buttonStyle(QuietButton())
                    .disabled(model.busy)
                    .help(m.title)
                    .accessibilityLabel("使用 \(m.title) 登录")
                }
            }

            Button(model.useToken ? "改用邮箱密码登录" : "改用访问令牌登录") {
                withAnimation(.easeInOut(duration: 0.18)) { model.useToken.toggle() }
            }
            .buttonStyle(.plain)
            .font(.caption)
            .foregroundStyle(Color.brand)

            // Said before the field is filled in, not after: "we store a token,
            // not your password" only reassures in advance.
            Text(model.useToken
                 ? "令牌保存在钥匙串，不写入配置文件"
                 : "密码只用来换取一枚这台设备专用的令牌，不会被保存")
                .font(.caption2).foregroundStyle(.tertiary)
                .multilineTextAlignment(.center)
        }
        .padding(26)
        .frame(width: 348)
        .panelSurface(18)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous).fill(.ultraThinMaterial)
        )
        .shadow(color: .black.opacity(0.45), radius: 28, y: 12)
    }

    private var header: some View {
        VStack(spacing: 9) {
            Image(systemName: "photo.on.rectangle.angled")
                .font(.system(size: 27, weight: .light))
                .foregroundStyle(Color.brandInk)
                .frame(width: 56, height: 56)
                .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Color.brand))
                .shadow(color: Color.brand.opacity(0.5), radius: 14, y: 5)

            // Ubuntu on the wordmark only — it has no CJK coverage, and letting
            // it fall through mid-sentence puts two typefaces in one line.
            HStack(spacing: 0) {
                Text("登录 ").font(.title2.weight(.semibold))
                Text("Open").font(.brand(21, .bold))
                Text("img").font(.brand(21, .bold)).foregroundStyle(Color.brand)
            }
            Text("图片托管与分发").font(.caption).foregroundStyle(.secondary)
        }
        .padding(.bottom, 2)
    }

    private var divider: some View {
        HStack(spacing: 10) {
            Rectangle().fill(.white.opacity(0.10)).frame(height: 1)
            Text("或").font(.caption2).foregroundStyle(.tertiary)
            Rectangle().fill(.white.opacity(0.10)).frame(height: 1)
        }
    }
}

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
            // and being a system symbol it carries no brand restrictions.
            Image(systemName: "touchid")
                .font(.system(size: 19))
                .foregroundStyle(Color.brand)
        }
    }
}
