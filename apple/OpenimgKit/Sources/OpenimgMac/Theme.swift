import SwiftUI

extension Color {
    /// The brand purple, same value as the site's violet anchors. Declared once
    /// so the app reads as the same product as the website rather than as
    /// generic system accent with a different icon.
    static let brand = Color(red: 0x79 / 255, green: 0x50 / 255, blue: 0xF2 / 255)
}

// MARK: - Glass

/// Liquid Glass where the OS provides it, a material where it does not.
///
/// `glassEffect` arrived in macOS 26. Raising the deployment target to match
/// would be simpler than these availability checks and would also mean the app
/// refuses to launch on Sonoma and Sequoia, which is most of the installed
/// base. `.regularMaterial` gives the same read — translucent chrome with the
/// content behind showing through — without the specular edge.
extension View {
    /// Chrome that floats over content: toolbars, status bars, popovers.
    @ViewBuilder
    func glassChrome<S: Shape>(_ shape: S) -> some View {
        if #available(macOS 26, *) {
            self.glassEffect(.regular, in: shape)
        } else {
            self.background(.regularMaterial, in: shape)
                .overlay(shape.stroke(.white.opacity(0.12), lineWidth: 0.5))
        }
    }

    /// Panels that hold content: dashboard cards, drop zones.
    ///
    /// Deliberately quieter than `glassChrome`. A card is a container, not a
    /// floating control, and giving every panel a specular rim turns a page
    /// into a pile of glass slabs with nothing to say which one matters.
    func glassCard(cornerRadius r: CGFloat = 14) -> some View {
        let shape = RoundedRectangle(cornerRadius: r, style: .continuous)
        return self
            .background(shape.fill(.background.opacity(0.55)))
            .background(shape.fill(.ultraThinMaterial))
            .overlay(shape.strokeBorder(.white.opacity(0.10), lineWidth: 0.8))
            .overlay(shape.strokeBorder(.primary.opacity(0.06), lineWidth: 0.8))
    }
}

// MARK: - Controls

/// The capsule filter row from the reference: one segment tinted, the rest
/// transparent until hovered.
struct PillPicker<T: Hashable & Identifiable>: View {
    let items: [T]
    let label: (T) -> String
    @Binding var selection: T

    var body: some View {
        HStack(spacing: 4) {
            ForEach(items) { item in
                Pill(text: label(item), active: item == selection) { selection = item }
            }
        }
        .padding(3)
        .glassChrome(Capsule())
    }
}

struct Pill: View {
    let text: String
    let active: Bool
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            Text(text)
                .font(.callout.weight(active ? .medium : .regular))
                .foregroundStyle(active ? Color.brand : .secondary)
                .padding(.horizontal, 13)
                .padding(.vertical, 5)
                .background {
                    Capsule().fill(
                        active ? Color.brand.opacity(0.16)
                               : Color.primary.opacity(hovering ? 0.06 : 0)
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

/// A card that lifts slightly under the pointer.
///
/// Hover feedback is what separates a grid of pictures from a grid of buttons:
/// without it there is nothing to say a card is clickable, and macOS users read
/// static tiles as decoration. Kept to 1.02 and a soft shadow — anything larger
/// makes a dense grid feel like it is breathing.
struct HoverLift: ViewModifier {
    @State private var hovering = false

    func body(content: Content) -> some View {
        content
            .scaleEffect(hovering ? 1.02 : 1)
            .shadow(color: .black.opacity(hovering ? 0.20 : 0), radius: hovering ? 9 : 0, y: 3)
            .animation(.easeOut(duration: 0.14), value: hovering)
            .onHover { hovering = $0 }
    }
}

extension View {
    func hoverLift() -> some View { modifier(HoverLift()) }
}

extension View {
    /// Lets the desktop show through the window chrome, which is what makes
    /// the glass read as glass rather than as grey. Landed in macOS 15; before
    /// that a window is simply opaque, and everything else still works.
    @ViewBuilder
    func windowGlass() -> some View {
        if #available(macOS 15, *) {
            self.containerBackground(.thinMaterial, for: .window)
        } else {
            self
        }
    }
}
