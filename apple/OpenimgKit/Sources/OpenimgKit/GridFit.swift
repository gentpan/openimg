import CoreGraphics
import Foundation

/// Works out the tile size that lays `count` items out to fill a given area.
///
/// The usual `GridItem(.adaptive(minimum:))` solves the opposite problem: it
/// fixes the tile and lets the row count fall where it may, so a page of 50 is
/// whatever height it happens to be and the window is either half empty or
/// scrolls. Here the page size is the thing the user chose, so the tile is the
/// free variable — pick the size that makes exactly that many fit.
///
/// Pure geometry, no SwiftUI, so `KitCheck` can assert on it.
public struct GridFit: Equatable, Sendable {
    public let columns: Int
    public let rows: Int
    public let cellWidth: Double
    public let cellHeight: Double
    /// True when the area was too small to fit `count` at a usable tile size,
    /// so the layout fell back to `minCell` and the grid has to scroll.
    public let scrolls: Bool

    /// How far a cell may depart from square.
    ///
    /// Filling both axes exactly is only a good idea when the result is roughly
    /// square. Three pictures in a wide window "fill" it as three 255x509
    /// slivers, which is worse than not filling. With a full page the solver
    /// lands near 1:1 on its own and this never binds; it exists for the tail.
    private static let minAspect = 0.78
    private static let maxAspect = 1.18

    /// - Parameters:
    ///   - count: how many tiles have to fit.
    ///   - size: the area available to the grid, insets already removed.
    ///   - spacing: the gap between tiles, both axes.
    ///   - minCell: the smallest tile worth drawing. Below this the grid gives
    ///     up on fitting and scrolls instead — 50 pictures at 40pt is not a
    ///     denser gallery, it is an unusable one.
    public static func solve(count: Int, in size: CGSize,
                             spacing: Double = 12, minCell: Double = 72) -> GridFit {
        let w = Double(size.width), h = Double(size.height)
        guard count > 0, w.isFinite, h.isFinite, w > 1, h > 1 else {
            return GridFit(columns: 1, rows: max(1, count),
                           cellWidth: minCell, cellHeight: minCell, scrolls: true)
        }

        var best: (c: Int, r: Int, w: Double, h: Double, tile: Double)?

        for c in 1...count {
            let r = Int(ceil(Double(count) / Double(c)))
            let cw = (w - spacing * Double(c - 1)) / Double(c)
            let ch = (h - spacing * Double(r - 1)) / Double(r)
            guard cw >= minCell, ch >= minCell else { continue }
            // Maximise the smaller side: a layout is only as good as the
            // dimension that ran out first.
            let tile = min(cw, ch)
            if best == nil || tile > best!.tile { best = (c, r, cw, ch, tile) }
        }

        guard let b = best else {
            // Nothing fits. Pack as many columns of `minCell` as the width
            // takes and let it run off the bottom.
            let c = max(1, Int((w + spacing) / (minCell + spacing)))
            let cw = max(minCell, (w - spacing * Double(c - 1)) / Double(c))
            return GridFit(columns: c, rows: Int(ceil(Double(count) / Double(c))),
                           cellWidth: cw, cellHeight: cw, scrolls: true)
        }

        let ch = min(max(b.h, b.w * minAspect), b.w * maxAspect)
        // Floored to whole points: `.fixed` columns that sum to a hair over the
        // proposed width make LazyVGrid drop a column, which would undo the
        // whole calculation over a rounding error.
        return GridFit(columns: b.c, rows: b.r,
                       cellWidth: floor(b.w), cellHeight: floor(ch), scrolls: false)
    }
}
