import SwiftUI
import Charts
import OpenimgKit

/// Swift Charts rather than a web view around the site's Chart.js.
///
/// Wrapping the existing charts in a WKWebView would have been less work and
/// would have shipped something that scrolls at a different speed to the rest
/// of the window, ignores the system accent, does not follow appearance
/// changes, and cannot be selected or right-clicked like anything else in the
/// app. The point of a native client is that it behaves like one.
struct OverviewView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ScrollView {
            // Two hand-assigned columns rather than a LazyVGrid.
            //
            // An adaptive grid centres every cell inside its row, and these
            // cards differ in height by a factor of four — so the row ended up
            // as five cards floating at five different heights with a hole down
            // the right. Columns also let related cards sit together instead of
            // wherever the flow happens to put them.
            HStack(alignment: .top, spacing: 16) {
                VStack(spacing: 16) {
                    quotaCard
                    compositionCard
                    formatCard
                }
                VStack(spacing: 16) {
                    checkinCard
                    ledgerCard
                }
            }
            .frame(maxWidth: 980)
            .frame(maxWidth: .infinity)   // centres the block in a wide window
            .padding(.horizontal, 22)
            .padding(.bottom, 22)
        }
        .task { await model.loadStats() }
    }

    // MARK: - Quota

    private var quotaCard: some View {
        Card("空间", "internaldrive") {
            if let q = model.quota {
                let used = q.quotaBytes > 0 ? Double(q.usedBytes) / Double(q.quotaBytes) : 0
                VStack(alignment: .leading, spacing: 10) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(model.bytes(q.availableBytes))
                            .font(.system(size: 30, weight: .semibold, design: .rounded))
                            .foregroundStyle(Color.accent)
                        Text("可用").foregroundStyle(.secondary)
                        Spacer()
                        // Moved here from the settings page, which carried a
                        // second copy of this whole card. Settings is for
                        // things you can change; nothing in it was.
                        Text("\(q.imageCount) 张")
                            .font(.callout).foregroundStyle(.secondary)
                    }
                    // A plain bar, not a gauge: this is one number against one
                    // ceiling, and a dial makes the reader do trigonometry to
                    // learn what a rectangle says immediately.
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            Capsule().fill(.quaternary)
                            Capsule()
                                .fill(Color.accent)
                                .frame(width: max(3, geo.size.width * min(1, used)))
                        }
                    }
                    .frame(height: 8)
                    HStack {
                        Text("已用 \(model.bytes(q.usedBytes))")
                        Spacer()
                        Text("总量 \(model.bytes(q.quotaBytes))")
                    }
                    .font(.caption).foregroundStyle(.secondary)
                }
            } else {
                placeholder
            }
        }
    }

    // MARK: - Storage composition

    private var compositionCard: some View {
        Card("存储构成", "chart.pie") {
            if let s = model.summary {
                // Distinct hues, not four opacities of one. Shades of the same
                // colour force the reader back to the legend for every slice;
                // and with 86 / 13 / 2 the two small ones were near-identical
                // slivers of purple that nothing distinguished.
                let parts = [
                    ("主图", s.sizePrimary, Color.accent),
                    ("衍生图", s.sizeVariants, Color(red: 0.20, green: 0.74, blue: 0.80)),
                    ("缩略图", s.sizeThumbs, Color(red: 0.98, green: 0.72, blue: 0.25)),
                    ("未分类", s.sizeUnclassified, Color.white.opacity(0.28)),
                ].filter { $0.1 > 0 }

                VStack(alignment: .leading, spacing: 12) {
                    Chart(parts, id: \.0) { part in
                        SectorMark(
                            angle: .value("占用", part.1),
                            innerRadius: .ratio(0.62),
                            angularInset: 1.5
                        )
                        .cornerRadius(3)
                        .foregroundStyle(part.2)
                    }
                    .chartLegend(.hidden)
                    .frame(height: 150)
                    .overlay {
                        VStack(spacing: 0) {
                            Text(model.bytes(s.sizeStored))
                                .font(.callout.weight(.semibold))
                            Text("\(s.images) 张").font(.caption2).foregroundStyle(.secondary)
                        }
                    }

                    ForEach(parts, id: \.0) { part in
                        HStack(spacing: 7) {
                            RoundedRectangle(cornerRadius: 2).fill(part.2).frame(width: 9, height: 9)
                            Text(part.0)
                            Spacer()
                            Text(model.bytes(part.1)).foregroundStyle(.secondary)
                            Text(percent(part.1, of: s.sizeStored))
                                .foregroundStyle(.tertiary)
                                .monospacedDigit()
                                .frame(width: 46, alignment: .trailing)
                        }
                        .font(.caption)
                    }

                    if s.sizeOrig > 0 {
                        Divider()
                        let delta = Double(s.sizeStored) / Double(s.sizeOrig) - 1
                        HStack {
                            Image(systemName: delta <= 0 ? "arrow.down.circle.fill" : "arrow.up.circle.fill")
                                .foregroundStyle(delta <= 0 ? .green : .orange)
                            Text(delta <= 0
                                 ? "比原始文件省了 \(model.bytes(s.sizeOrig - s.sizeStored))"
                                 : "比原始文件多用 \(model.bytes(s.sizeStored - s.sizeOrig))")
                            Spacer()
                            Text(String(format: "%+.0f%%", delta * 100)).monospacedDigit()
                        }
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    }
                }
            } else {
                placeholder
            }
        }
    }

    // MARK: - Formats

    private var formatCard: some View {
        Card("格式分布", "doc.on.doc") {
            if let s = model.summary, !s.byFormat.isEmpty {
                Chart(Array(s.byFormat.enumerated()), id: \.element.id) { i, f in
                    BarMark(
                        x: .value("占用", f.bytes),
                        y: .value("格式", f.ext.uppercased())
                    )
                    .foregroundStyle(Self.formatPalette[i % Self.formatPalette.count])
                    .cornerRadius(4)
                    .annotation(position: .trailing) {
                        Text(model.bytes(f.bytes))
                            .font(.caption2).foregroundStyle(.secondary)
                    }
                }
                .chartXAxis(.hidden)
                // Room on the right for the annotation, which would otherwise
                // be clipped by the plot area on the widest bar.
                .chartXScale(range: .plotDimension(endPadding: 66))
                .frame(height: CGFloat(s.byFormat.count) * 30 + 12)
            } else {
                placeholder
            }
        }
    }

    // MARK: - Check-in

    private var checkinCard: some View {
        Card("签到", "flame.fill") {
            VStack(alignment: .leading, spacing: 16) {
                HStack(alignment: .center, spacing: 12) {
                    Image(systemName: "flame.fill")
                        .font(.system(size: 22))
                        .foregroundStyle(.orange)
                        .frame(width: 44, height: 44)
                        .background(Circle().fill(.orange.opacity(0.15)))
                        .shadow(color: .orange.opacity(0.35), radius: 8)
                    VStack(alignment: .leading, spacing: 0) {
                        Text("连续签到").font(.caption).foregroundStyle(.secondary)
                        HStack(alignment: .firstTextBaseline, spacing: 4) {
                            Text("\(model.streak)")
                                .font(.system(size: 26, weight: .semibold, design: .rounded))
                            Text("天").font(.callout).foregroundStyle(.secondary)
                        }
                    }
                    Spacer()
                    Button(model.checkedInToday ? "今天已签到" : "签到") {
                        Task { await model.checkin() }
                    }
                    .buttonStyle(model.checkedInToday ? AnyButtonStyle(QuietButton())
                                                      : AnyButtonStyle(BrandButton()))
                    .disabled(model.checkedInToday || model.busy)
                }

                Divider().overlay(Color.white.opacity(0.08))

                WeekStrip(days: model.thisWeek)
                MilestoneBar(
                    title: "本月进度",
                    current: model.monthProgress.done,
                    total: model.monthProgress.total,
                    reward: model.quota?.checkin?.monthBonus ?? 0,
                    bytes: model.bytes
                )
            }
        }
    }

    // MARK: - Ledger

    private var ledgerCard: some View {
        Card("空间流水", "list.bullet.rectangle") {
            if model.transactions.isEmpty {
                placeholder
            } else {
                VStack(spacing: 0) {
                    ForEach(model.transactions.prefix(12)) { t in
                        HStack(spacing: 10) {
                            Image(systemName: t.isGrant ? "plus.circle.fill" : "minus.circle.fill")
                                .foregroundStyle(t.isGrant ? Color.accent : Color.secondary)
                                .font(.caption)
                            VStack(alignment: .leading, spacing: 1) {
                                Text(t.label).font(.caption)
                                Text(t.reason)
                                    .font(.caption2).foregroundStyle(.tertiary)
                                    .lineLimit(1).truncationMode(.middle)
                            }
                            Spacer()
                            Text("\(t.isGrant ? "+" : "−")\(model.bytes(t.bytes))")
                                .font(.caption.monospacedDigit())
                                .foregroundStyle(t.isGrant ? Color.accent : .secondary)
                        }
                        .padding(.vertical, 5)
                        if t.id != model.transactions.prefix(12).last?.id { Divider() }
                    }
                }
            }
        }
    }

    // MARK: - Bits

    private var placeholder: some View {
        HStack {
            Spacer()
            if model.statsLoading { ProgressView().controlSize(.small) }
            else { Text("暂无数据").font(.caption).foregroundStyle(.tertiary) }
            Spacer()
        }
        .frame(height: 90)
    }

    /// Cycled across formats so the bars are told apart by colour as well as
    /// by length — the shortest two are otherwise a pair of stubs.
    static let formatPalette: [Color] = [
        // Accent first, not brand: a chart answers "how much", and violet in a
        // chart reads as "this series is the brand one" rather than as a
        // colour. The fourth used to be another green and sat too close to the
        // first — a slate replaces it, since it only appears once there are
        // four or more formats and a low-saturation neutral cannot clash.
        .accent,
        Color(red: 0.22, green: 0.74, blue: 0.97),
        Color(red: 0.98, green: 0.75, blue: 0.14),
        Color(red: 0.58, green: 0.64, blue: 0.72),
        Color(red: 0.96, green: 0.45, blue: 0.71),
    ]

    private func percent(_ part: Int64, of whole: Int64) -> String {
        guard whole > 0 else { return "—" }
        return String(format: "%.0f%%", Double(part) / Double(whole) * 100)
    }
}

// MARK: - Card chrome

private struct Card<Content: View>: View {
    let title: String
    let icon: String
    @ViewBuilder let content: Content

    init(_ title: String, _ icon: String, @ViewBuilder content: () -> Content) {
        self.title = title
        self.icon = icon
        self.content = content()
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

/// GitHub-style contribution grid for the check-in streak.
