import SwiftUI
import OpenimgKit

/// A day in the current week, and whether it was claimed.
struct CheckinDay: Identifiable {
    let id: String        // yyyy-MM-dd
    let label: String     // 一 二 三 …
    let claimed: Bool
    let isToday: Bool
    let future: Bool
}

/// Mon–Sun as filled circles, the reference's shape.
///
/// A week is the unit people actually think in, and it is also the smaller of
/// the two milestones — so it is the one worth showing day by day. The month
/// gets a bar, because thirty circles is a grid, not a glance.
struct WeekStrip: View {
    let days: [CheckinDay]

    var body: some View {
        HStack(spacing: 0) {
            ForEach(days) { d in
                VStack(spacing: 6) {
                    ZStack {
                        Circle()
                            .fill(d.claimed ? Color.brand : .white.opacity(0.07))
                        if d.isToday && !d.claimed {
                            // Today is outlined rather than filled: it is the
                            // one still available, and a hollow ring reads as
                            // "your turn" where a grey disc reads as "missed".
                            Circle().strokeBorder(Color.brand, lineWidth: 2)
                        }
                        if d.claimed {
                            Image(systemName: "checkmark")
                                .font(.system(size: 11, weight: .bold))
                                .foregroundStyle(.white)
                        }
                    }
                    .frame(width: 30, height: 30)
                    .opacity(d.future ? 0.45 : 1)

                    Text(d.label)
                        .font(.caption2)
                        .foregroundStyle(d.isToday ? Color.brand : .secondary)
                }
                .frame(maxWidth: .infinity)
            }
        }
    }
}

/// Progress toward a milestone, with what it pays stated up front.
struct MilestoneBar: View {
    let title: String
    let current: Int
    let total: Int
    let reward: Int64
    let bytes: (Int64) -> String

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(title).font(.caption).foregroundStyle(.secondary)
                Spacer()
                if reward > 0 {
                    // The number is the point. "Keep your streak" means nothing
                    // without saying what the streak is worth.
                    Text("满 \(total) 天 +\(bytes(reward))")
                        .font(.caption).foregroundStyle(Color.brand)
                }
            }
            ProgressBar(value: total > 0 ? Double(current) / Double(total) : 0)
                .frame(height: 6)
            Text("\(current) / \(total) 天")
                .font(.caption2).foregroundStyle(.tertiary)
        }
    }
}

/// Lets a call site pick between button styles at runtime.
struct AnyButtonStyle: ButtonStyle {
    private let make: (Configuration) -> AnyView
    init<S: ButtonStyle>(_ style: S) {
        make = { AnyView(style.makeBody(configuration: $0)) }
    }
    func makeBody(configuration: Configuration) -> some View { make(configuration) }
}
