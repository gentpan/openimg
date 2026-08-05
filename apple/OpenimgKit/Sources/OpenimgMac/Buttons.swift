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
            .font(.callout.weight(.semibold))
            // Disabled takes a neutral fill rather than a faded brand one:
            // fading only the background leaves the label at full strength on a
            // surface that no longer supports it.
            .foregroundStyle(enabled ? AnyShapeStyle(Color.brandInk)
                                     : AnyShapeStyle(.tertiary))
            .padding(.horizontal, 15)
            .padding(.vertical, 7)
            .background(
                RoundedRectangle(cornerRadius: 9, style: .continuous)
                    .fill(enabled ? AnyShapeStyle(Color.brand)
                                  : AnyShapeStyle(Color.white.opacity(0.08)))
                    .opacity(configuration.isPressed ? 0.78 : 1)
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


/// A text-only action, in the brand colour.
///
/// `.buttonStyle(LinkButton())` would be the one-liner and it paints with the *system*
/// accent — blue on a default Mac — which is the one colour this product does
/// not use. Small enough that a full button would shout, so it stays text.
struct LinkButton: ButtonStyle {
    @Environment(\.isEnabled) private var enabled
    @State private var hovering = false

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .foregroundStyle(enabled ? AnyShapeStyle(Color.brand) : AnyShapeStyle(.tertiary))
            .opacity(configuration.isPressed ? 0.6 : 1)
            .underline(hovering && enabled)
            .contentShape(Rectangle())
            .onHover { hovering = $0 }
            .animation(.easeOut(duration: 0.1), value: hovering)
    }
}
