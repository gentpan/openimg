import SwiftUI

extension Color {
    /// The brand purple, same value as `--color-violet-*` anchors on the site.
    /// Declared once here so the app reads as the same product as the website
    /// rather than as generic system blue with a different icon.
    static let brand = Color(red: 0x79 / 255, green: 0x50 / 255, blue: 0xF2 / 255)
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
            .shadow(color: .black.opacity(hovering ? 0.18 : 0), radius: hovering ? 8 : 0, y: 3)
            .animation(.easeOut(duration: 0.14), value: hovering)
            .onHover { hovering = $0 }
    }
}

extension View {
    func hoverLift() -> some View { modifier(HoverLift()) }
}
