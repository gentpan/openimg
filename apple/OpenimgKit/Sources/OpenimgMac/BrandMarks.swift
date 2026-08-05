import SwiftUI

/// Google and GitHub sign-in marks, drawn from their official path data.
///
/// The previous version used a letter "G" in a circle and an SF Symbol of
/// angle brackets. Both were placeholders that happened to compile: neither is
/// the mark users recognise, and a sign-in button that does not look like the
/// provider is one people hesitate to press. Brand guidelines for both also
/// require the actual mark rather than an approximation.
///
/// Drawn rather than bundled as images so they stay sharp at any size and
/// follow the appearance without needing a light and a dark asset each.

// MARK: - SVG path parsing

/// A minimal SVG path reader, enough for these two marks.
///
/// Hand-transcribing beziers into `Path` calls is where transcription errors
/// hide — a wrong control point yields a mark that is subtly off rather than
/// obviously broken. Keeping the original `d` strings verbatim means the data
/// is checkable against the source it came from.
struct SVGPath: Shape {
    let commands: String
    /// The coordinate system the `d` string was authored in.
    let viewBox: CGFloat

    func path(in rect: CGRect) -> Path {
        var p = Path()
        var scanner = PathScanner(commands)
        var cur = CGPoint.zero
        var start = CGPoint.zero
        var lastControl: CGPoint?
        var op: Character = "M"

        while let next = scanner.nextOperatorOrNumber() {
            if case .op(let c) = next { op = c } else { scanner.pushBack() }
            let relative = op.isLowercase
            let o = Character(op.uppercased())

            func pt(_ x: CGFloat, _ y: CGFloat) -> CGPoint {
                relative ? CGPoint(x: cur.x + x, y: cur.y + y) : CGPoint(x: x, y: y)
            }

            switch o {
            case "M":
                guard let x = scanner.number(), let y = scanner.number() else { return p }
                cur = pt(x, y); start = cur; p.move(to: cur)
                op = relative ? "l" : "L" // implicit lineto for extra pairs
            case "L":
                guard let x = scanner.number(), let y = scanner.number() else { return p }
                cur = pt(x, y); p.addLine(to: cur)
            case "H":
                guard let x = scanner.number() else { return p }
                cur = CGPoint(x: relative ? cur.x + x : x, y: cur.y); p.addLine(to: cur)
            case "V":
                guard let y = scanner.number() else { return p }
                cur = CGPoint(x: cur.x, y: relative ? cur.y + y : y); p.addLine(to: cur)
            case "C":
                guard let a = scanner.number(), let b = scanner.number(),
                      let c = scanner.number(), let d = scanner.number(),
                      let e = scanner.number(), let f = scanner.number() else { return p }
                let c1 = pt(a, b), c2 = pt(c, d), end = pt(e, f)
                p.addCurve(to: end, control1: c1, control2: c2)
                lastControl = c2; cur = end
            case "S":
                guard let c = scanner.number(), let d = scanner.number(),
                      let e = scanner.number(), let f = scanner.number() else { return p }
                let c1 = lastControl.map { CGPoint(x: 2 * cur.x - $0.x, y: 2 * cur.y - $0.y) } ?? cur
                let c2 = pt(c, d), end = pt(e, f)
                p.addCurve(to: end, control1: c1, control2: c2)
                lastControl = c2; cur = end
            case "A":
                // Both marks use arcs, and skipping them does not fail loudly:
                // the path simply stops mid-outline and renders as a plausible
                // but wrong shape, which is exactly the failure a brand mark
                // must not have.
                guard let rx = scanner.number(), let ry = scanner.number(),
                      let rot = scanner.number(), let large = scanner.number(),
                      let sweep = scanner.number(),
                      let x = scanner.number(), let y = scanner.number() else { return p }
                let end = pt(x, y)
                appendArc(&p, from: cur, to: end, rx: rx, ry: ry,
                          rotation: rot, largeArc: large != 0, sweep: sweep != 0)
                cur = end
            case "Z":
                p.closeSubpath(); cur = start
            default:
                return p // unsupported verb: stop rather than draw nonsense
            }
            if o != "C" && o != "S" { lastControl = nil }
        }

        let s = min(rect.width, rect.height) / viewBox
        return p.applying(CGAffineTransform(scaleX: s, y: s))
    }
}

/// SVG states an arc by its endpoint; Core Graphics wants a centre. This is
/// the conversion from the SVG specification's implementation notes (F.6.5),
/// followed by splitting the sweep into ≤90° cubic segments — a bezier cannot
/// represent a quarter turn of an ellipse exactly, and beyond that the error
/// becomes visible.
private func appendArc(_ p: inout Path, from: CGPoint, to: CGPoint,
                       rx rxIn: CGFloat, ry ryIn: CGFloat,
                       rotation: CGFloat, largeArc: Bool, sweep: Bool) {
    var rx = abs(rxIn), ry = abs(ryIn)
    if rx == 0 || ry == 0 { p.addLine(to: to); return }   // degenerate: a line

    let phi = rotation * .pi / 180
    let cosPhi = cos(phi), sinPhi = sin(phi)
    let dx2 = (from.x - to.x) / 2, dy2 = (from.y - to.y) / 2
    let x1p = cosPhi * dx2 + sinPhi * dy2
    let y1p = -sinPhi * dx2 + cosPhi * dy2

    // Scale up radii that are too small to span the endpoints (F.6.6).
    let lambda = (x1p * x1p) / (rx * rx) + (y1p * y1p) / (ry * ry)
    if lambda > 1 { rx *= sqrt(lambda); ry *= sqrt(lambda) }

    let sign: CGFloat = largeArc == sweep ? -1 : 1
    let num = max(0, rx * rx * ry * ry - rx * rx * y1p * y1p - ry * ry * x1p * x1p)
    let den = rx * rx * y1p * y1p + ry * ry * x1p * x1p
    let coef = sign * sqrt(den == 0 ? 0 : num / den)
    let cxp = coef * rx * y1p / ry
    let cyp = -coef * ry * x1p / rx
    let cx = cosPhi * cxp - sinPhi * cyp + (from.x + to.x) / 2
    let cy = sinPhi * cxp + cosPhi * cyp + (from.y + to.y) / 2

    func angle(_ ux: CGFloat, _ uy: CGFloat) -> CGFloat { atan2(uy, ux) }
    let theta1 = angle((x1p - cxp) / rx, (y1p - cyp) / ry)
    var delta = angle((-x1p - cxp) / rx, (-y1p - cyp) / ry) - theta1
    if !sweep && delta > 0 { delta -= 2 * .pi }
    if sweep && delta < 0 { delta += 2 * .pi }

    let segments = max(1, Int(ceil(abs(delta) / (.pi / 2))))
    let step = delta / CGFloat(segments)
    // Magic constant for approximating a circular arc with a cubic.
    let k = 4.0 / 3.0 * tan(step / 4)

    var t = theta1
    for _ in 0..<segments {
        let t2 = t + step
        let (c1, s1) = (cos(t), sin(t))
        let (c2, s2) = (cos(t2), sin(t2))

        func map(_ x: CGFloat, _ y: CGFloat) -> CGPoint {
            CGPoint(x: cosPhi * rx * x - sinPhi * ry * y + cx,
                    y: sinPhi * rx * x + cosPhi * ry * y + cy)
        }
        p.addCurve(
            to: map(c2, s2),
            control1: map(c1 - k * s1, s1 + k * c1),
            control2: map(c2 + k * s2, s2 - k * c2)
        )
        t = t2
    }
}

private struct PathScanner {
    private let chars: [Character]
    private var i = 0
    private var lastIndex = 0

    init(_ s: String) { chars = Array(s) }

    enum Token { case op(Character); case num(CGFloat) }

    mutating func pushBack() { i = lastIndex }

    mutating func nextOperatorOrNumber() -> Token? {
        skip()
        lastIndex = i
        guard i < chars.count else { return nil }
        if chars[i].isLetter { defer { i += 1 }; return .op(chars[i]) }
        return number().map { .num($0) }
    }

    mutating func number() -> CGFloat? {
        skip()
        var s = ""
        if i < chars.count, chars[i] == "-" || chars[i] == "+" { s.append(chars[i]); i += 1 }
        while i < chars.count, chars[i].isNumber || chars[i] == "." {
            // A second dot starts the next number: "1.5.5" is two values.
            if chars[i] == ".", s.contains(".") { break }
            s.append(chars[i]); i += 1
        }
        return Double(s).map { CGFloat($0) }
    }

    private mutating func skip() {
        while i < chars.count, chars[i] == " " || chars[i] == "," || chars[i] == "\n" { i += 1 }
    }
}

// MARK: - Marks

/// The four-colour Google G. Each colour is its own subpath in the official
/// artwork, which is why this is four shapes rather than one.
struct GoogleMark: View {
    var body: some View {
        ZStack {
            SVGPath(commands: "M23.49 12.27c0-.79-.07-1.54-.19-2.27H12v4.51h6.47a5.4 5.4 0 0 1-2.4 3.58v3h3.86c2.26-2.09 3.56-5.17 3.56-8.82Z", viewBox: 24)
                .fill(Color(red: 0.259, green: 0.522, blue: 0.957))
            SVGPath(commands: "M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.86-3c-1.08.72-2.45 1.16-4.07 1.16-3.13 0-5.78-2.11-6.73-4.96H1.29v3.09A11.99 11.99 0 0 0 12 24Z", viewBox: 24)
                .fill(Color(red: 0.204, green: 0.659, blue: 0.325))
            SVGPath(commands: "M5.27 14.29c-.25-.72-.38-1.49-.38-2.29s.14-1.57.38-2.29V6.62H1.29A11.99 11.99 0 0 0 0 12c0 1.94.46 3.77 1.29 5.38l3.98-3.09Z", viewBox: 24)
                .fill(Color(red: 0.984, green: 0.737, blue: 0.020))
            SVGPath(commands: "M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.31 0 3.26 2.69 1.29 6.62l3.98 3.09C6.22 6.86 8.87 4.75 12 4.75Z", viewBox: 24)
                .fill(Color(red: 0.918, green: 0.263, blue: 0.208))
        }
    }
}

/// The GitHub Octocat, same path the site footer uses.
struct GitHubMark: View {
    var body: some View {
        SVGPath(
            commands: "M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z",
            viewBox: 16
        )
        .fill(.primary)
    }
}
