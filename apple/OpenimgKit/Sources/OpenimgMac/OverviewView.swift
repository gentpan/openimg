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

    private let cardColumns = [GridItem(.adaptive(minimum: 330, maximum: 560), spacing: 16)]

    var body: some View {
        ScrollView {
            LazyVGrid(columns: cardColumns, alignment: .leading, spacing: 16) {
                quotaCard
                compositionCard
                formatCard
                checkinCard
                ledgerCard
            }
            .padding(18)
        }
        .task { await model.loadStats() }
        .refreshable { await model.loadStats() }
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
                            .foregroundStyle(Color.brand)
                        Text("可用").foregroundStyle(.secondary)
                    }
                    // A plain bar, not a gauge: this is one number against one
                    // ceiling, and a dial makes the reader do trigonometry to
                    // learn what a rectangle says immediately.
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            Capsule().fill(.quaternary)
                            Capsule()
                                .fill(LinearGradient(colors: [Color.brand.opacity(0.7), Color.brand],
                                                     startPoint: .leading, endPoint: .trailing))
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
                let parts = [
                    ("主图", s.sizePrimary, Color.brand),
                    ("衍生图", s.sizeVariants, Color.brand.opacity(0.62)),
                    ("缩略图", s.sizeThumbs, Color.brand.opacity(0.34)),
                    ("未分类", s.sizeUnclassified, Color.secondary.opacity(0.35)),
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
                Chart(s.byFormat) { f in
                    BarMark(
                        x: .value("占用", f.bytes),
                        y: .value("格式", f.ext.uppercased())
                    )
                    .foregroundStyle(Color.brand.gradient)
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
        Card("签到", "calendar") {
            VStack(alignment: .leading, spacing: 12) {
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text("\(model.streak)")
                        .font(.system(size: 30, weight: .semibold, design: .rounded))
                        .foregroundStyle(Color.brand)
                    Text("天连续").foregroundStyle(.secondary)
                    Spacer()
                    Button(model.checkedInToday ? "今天已签到" : "签到") {
                        Task { await model.checkin() }
                    }
                    .buttonStyle(BrandButton())
                    .disabled(model.checkedInToday || model.busy)
                }
                HeatCalendar(records: model.checkins)
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
                                .foregroundStyle(t.isGrant ? Color.brand : Color.secondary)
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
                                .foregroundStyle(t.isGrant ? Color.brand : .secondary)
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
private struct HeatCalendar: View {
    let records: [CheckinRecord]
    private let weeks = 17

    var body: some View {
        let byDay = Dictionary(uniqueKeysWithValues: records.map { ($0.date, $0) })
        let days = grid()
        let maxBytes = max(1, records.map(\.bytes).max() ?? 1)

        VStack(alignment: .leading, spacing: 5) {
            HStack(spacing: 3) {
                ForEach(0..<weeks, id: \.self) { w in
                    VStack(spacing: 3) {
                        ForEach(0..<7, id: \.self) { d in
                            let idx = w * 7 + d
                            let key = idx < days.count ? days[idx] : ""
                            cell(for: byDay[key], date: key, max: maxBytes)
                        }
                    }
                }
            }
            HStack(spacing: 4) {
                Text("少").font(.caption2).foregroundStyle(.tertiary)
                ForEach([0.15, 0.35, 0.6, 1.0], id: \.self) { level in
                    RoundedRectangle(cornerRadius: 2)
                        .fill(Color.brand.opacity(level))
                        .frame(width: 9, height: 9)
                }
                Text("多").font(.caption2).foregroundStyle(.tertiary)
            }
        }
    }

    private func cell(for rec: CheckinRecord?, date: String, max maxBytes: Int64) -> some View {
        let level = rec.map { 0.15 + 0.85 * (Double($0.bytes) / Double(maxBytes)) } ?? 0
        return RoundedRectangle(cornerRadius: 2)
            .fill(rec == nil ? AnyShapeStyle(.quaternary) : AnyShapeStyle(Color.brand.opacity(level)))
            .frame(width: 11, height: 11)
            .help(rec == nil ? date : "\(date)　+\(ByteCountFormatter.string(fromByteCount: rec!.bytes, countStyle: .binary))")
    }

    /// Dates for the grid, oldest first, ending today — the server stores days
    /// as UTC `yyyy-mm-dd`, so the calendar has to match or the columns drift.
    private func grid() -> [String] {
        var cal = Calendar(identifier: .gregorian)
        cal.timeZone = TimeZone(identifier: "UTC")!
        let fmt = DateFormatter()
        fmt.dateFormat = "yyyy-MM-dd"
        fmt.timeZone = cal.timeZone

        let today = Date()
        return (0..<(weeks * 7)).reversed().compactMap { back in
            cal.date(byAdding: .day, value: -back, to: today).map(fmt.string(from:))
        }
    }
}
