import SwiftUI

/// One button vocabulary for the whole app.
///
/// Before this each view reached for `.borderedProminent`, `.bordered` or
/// `.plain` as it went, so the same weight of action looked different on every
/// page — which is most of what made the app read as rough rather than as one
/// product. Three roles, used consistently: brand for the one action a screen
/// is for, quiet for everything else, danger for the irreversible.
struct BrandButton: ButtonStyle {
    @Environment(\.isEnabled) private var enabled
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.callout.weight(.medium))
            .foregroundStyle(.white)
            .padding(.horizontal, 15)
            .padding(.vertical, 7)
            .background(
                RoundedRectangle(cornerRadius: 9, style: .continuous)
                    .fill(Color.brand)
                    .opacity(enabled ? (configuration.isPressed ? 0.78 : 1) : 0.35)
            )
            .animation(.easeOut(duration: 0.1), value: configuration.isPressed)
    }
}

struct QuietButton: ButtonStyle {
    @Environment(\.isEnabled) private var enabled
    @State private var hovering = false

    func makeBody(configuration: Configuration) -> some View {
        let shape = RoundedRectangle(cornerRadius: 9, style: .continuous)
        return configuration.label
            .font(.callout)
            .foregroundStyle(enabled ? .primary : .tertiary)
            .padding(.horizontal, 13)
            .padding(.vertical, 6)
            .background(shape.fill(.white.opacity(
                configuration.isPressed ? 0.14 : hovering && enabled ? 0.09 : 0.05
            )))
            .overlay(shape.strokeBorder(.white.opacity(0.09), lineWidth: 0.8))
            .onHover { hovering = $0 }
            .animation(.easeOut(duration: 0.1), value: hovering)
    }
}

struct DangerButton: ButtonStyle {
    @State private var hovering = false
    func makeBody(configuration: Configuration) -> some View {
        let shape = RoundedRectangle(cornerRadius: 9, style: .continuous)
        return configuration.label
            .font(.callout.weight(.medium))
            .foregroundStyle(Color(red: 1, green: 0.42, blue: 0.42))
            .padding(.horizontal, 13)
            .padding(.vertical, 6)
            .background(shape.fill(.red.opacity(configuration.isPressed ? 0.24 : hovering ? 0.18 : 0.12)))
            .overlay(shape.strokeBorder(.red.opacity(0.28), lineWidth: 0.8))
            .onHover { hovering = $0 }
            .animation(.easeOut(duration: 0.1), value: hovering)
    }
}

/// The one text-field shape used everywhere, so login, search and settings do
/// not each invent their own.
struct Field<C: View>: View {
    let icon: String
    @ViewBuilder let content: C

    var body: some View {
        HStack(spacing: 9) {
            Image(systemName: icon)
                .font(.system(size: 12))
                .foregroundStyle(.tertiary)
                .frame(width: 15)
            content
                .textFieldStyle(.plain)
                .font(.callout)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .background(
            RoundedRectangle(cornerRadius: 9, style: .continuous).fill(.white.opacity(0.06))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 9, style: .continuous)
                .strokeBorder(.white.opacity(0.10), lineWidth: 0.8)
        )
    }
}
