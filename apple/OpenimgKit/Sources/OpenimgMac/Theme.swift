import SwiftUI

/// One scale, and one rule about what goes on it.
///
/// #5DE31D has a relative luminance of 0.574, so white on it is 1.68:1 —
/// unreadable, not merely low. Every filled control therefore takes
/// `Color.brandInk` (10.65:1), and white never appears on a brand surface.
/// That single fact is the whole difference from the violet this replaces,
/// which was 0.135 and carried white at 4.67:1.
/// One height for every control that can sit next to another one.
///
/// Padding maths gave the toolbar three different heights — a pill row at 32,
/// a quiet button at 28, a brand button at 30 — which is invisible in isolation
/// and obvious the moment two of them share a row. A fixed height is also
/// stable across fonts, where padding is not.
enum Metrics {
    static let control: CGFloat = 32
}

extension Color {
    /// Same value as the site's `--color-brand-600`.
    static let brand = Color(red: 0x5D / 255, green: 0xE3 / 255, blue: 0x1D / 255)

    /// What goes *on* a brand fill. Near-black with a trace of the brand hue,
    /// so it reads as part of the control rather than as borrowed body text.
    /// The grey the toolbar's plain tiles resolve to, as a concrete Color so it
    /// can be handed to `.tint`, which does not take a hierarchical style.
    static let secondaryLabel = Color(nsColor: .secondaryLabelColor)

    static let brandInk = Color(red: 0x0A / 255, green: 0x1B / 255, blue: 0x02 / 255)

    /// "It worked" — kept at a distinctly different hue (teal, 173°) from the
    /// brand (101°) so a success message never reads as brand chrome.
    static let success = Color(red: 0x14 / 255, green: 0xB8 / 255, blue: 0xA6 / 255)
}

// MARK: - Surfaces
//
// The app is dark regardless of the system setting, and that is the whole
// premise rather than a preference. Translucent chrome only reads as glass over
// something darker than itself; on a light desktop the same materials stack up
// into a white wall, which is exactly what the first attempt produced. Every
// value below assumes a dark base, so following the system appearance would
// mean maintaining two sets of them.

extension View {
    /// The window's own backdrop. Dark enough that the layers above it have
    /// something to separate from, translucent enough to pick up the desktop.
    func windowSurface() -> some View {
        background {
            ZStack {
                Rectangle().fill(.ultraThinMaterial)
                Rectangle().fill(Color.black.opacity(0.38))
            }
            .ignoresSafeArea()
        }
    }

    /// Floating chrome: toolbar clusters, pill rows, the toast.
    ///
    /// A visible top edge and a darker fill, because on a dark base a shadow
    /// alone does not separate anything — the lift has to come from the rim
    /// catching light, which is what the reference does.
    func chromeSurface<S: InsettableShape>(_ shape: S, elevated: Bool = true) -> some View {
        let tint: Color = .black.opacity(elevated ? 0.28 : 0.16)
        return self
            .background(shape.fill(tint))
            .background(shape.fill(.ultraThinMaterial))
            .overlay(shape.strokeBorder(Color.white.opacity(0.10), lineWidth: 0.8))
    }

    /// Panels that hold content: dashboard cards, drop zones, the login card.
    /// Quieter than chrome — a container should not compete with the controls
    /// floating over it.
    func panelSurface(_ radius: CGFloat = 14) -> some View {
        let shape = RoundedRectangle(cornerRadius: radius, style: .continuous)
        return self
            .background(shape.fill(Color.white.opacity(0.045)))
            .overlay(shape.strokeBorder(Color.white.opacity(0.08), lineWidth: 0.8))
    }

    /// The reading area to the right of the sidebar, one step lighter so the
    /// two columns read as distinct planes without a divider between them.
    func contentSurface() -> some View {
        background(Color.white.opacity(0.03))
    }
}

// MARK: - Pills

/// The capsule filter row from the reference: the active segment carries a
/// solid fill, the rest stay transparent until hovered.
struct PillRow<T: Hashable & Identifiable>: View {
    let items: [T]
    let label: (T) -> String
    var icon: ((T) -> String)? = nil
    @Binding var selection: T

    var body: some View {
        HStack(spacing: 3) {
            ForEach(items) { item in
                Pill(
                    text: label(item),
                    icon: icon?(item),
                    active: item == selection
                ) { selection = item }
            }
        }
        .padding(3)
        .chromeSurface(Capsule(), elevated: false)
    }
}

struct Pill: View {
    let text: String
    var icon: String? = nil
    let active: Bool
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 5) {
                if let icon { Image(systemName: icon).font(.system(size: 10.5)) }
                Text(text)
            }
            .font(.callout.weight(active ? .medium : .regular))
            .foregroundStyle(active ? AnyShapeStyle(Color.brandInk) : AnyShapeStyle(.secondary))
            .padding(.horizontal, 12)
            .frame(height: Metrics.control - 6)   // 减去 PillRow 外圈的 3pt 内边距
            .background {
                Capsule().fill(
                    active ? AnyShapeStyle(Color.brand)
                           : AnyShapeStyle(Color.white.opacity(hovering ? 0.08 : 0))
                )
            }
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .animation(.easeOut(duration: 0.12), value: active)
        .animation(.easeOut(duration: 0.12), value: hovering)
    }
}

/// A toolbar button shaped like the reference's: a square-ish glass tile that
/// groups with its neighbours rather than a bare system button.
struct ToolTile: View {
    let icon: String
    var help: String = ""
    var disabled = false
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            Image(systemName: icon)
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(disabled ? .tertiary : .secondary)
                .frame(width: 30, height: Metrics.control - 6)
                .background {
                    RoundedRectangle(cornerRadius: 7, style: .continuous)
                        .fill(.white.opacity(hovering && !disabled ? 0.10 : 0))
                }
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .help(help)
        .onHover { hovering = $0 }
        .animation(.easeOut(duration: 0.1), value: hovering)
    }
}

/// Wraps a run of tiles into one floating cluster.
struct ToolCluster<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        HStack(spacing: 1) { content }
            .padding(3)
            .chromeSurface(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }
}

// MARK: - Hover

/// A card that lifts slightly under the pointer. Without it a grid of pictures
/// reads as decoration rather than as something clickable.
struct HoverLift: ViewModifier {
    @State private var hovering = false
    func body(content: Content) -> some View {
        content
            .scaleEffect(hovering ? 1.02 : 1)
            .shadow(color: .black.opacity(hovering ? 0.35 : 0), radius: hovering ? 10 : 0, y: 4)
            .animation(.easeOut(duration: 0.14), value: hovering)
            .onHover { hovering = $0 }
    }
}

extension View {
    func hoverLift() -> some View { modifier(HoverLift()) }
}

// MARK: - Brand typeface

/// Ubuntu, bundled with the app. The site sets its wordmark in Ubuntu, and a
/// client that renders the same name in the system face reads as a different
/// product. Latin only, so it goes on the wordmark rather than whole strings.
enum BrandFont {
    static func register() {
        for name in ["Ubuntu-Regular", "Ubuntu-Medium", "Ubuntu-Bold"] {
            guard let url = Bundle.main.url(forResource: name, withExtension: "ttf") else { continue }
            CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
        }
    }
}

extension Font {
    static func brand(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        let face = switch weight {
        case .bold, .heavy, .black: "Ubuntu-Bold"
        case .medium, .semibold: "Ubuntu-Medium"
        default: "Ubuntu"
        }
        return NSFont(name: face, size: size) != nil
            ? .custom(face, size: size)
            : .system(size: size, weight: weight, design: .rounded)
    }
}
