import SwiftUI
import AppKit
import OpenimgKit

struct GalleryView: View {
    @ObservedObject var model: AppModel
    private static let spacing: Double = 12

    var body: some View {
        VStack(spacing: 0) {
            if !model.images.isEmpty { filters }

            if model.images.isEmpty {
                EmptyState(model: model)
            } else {
                grid
            }
            statusBar
        }
        // A full-window overlay rather than an inspector column: the point of
        // opening a picture is to see it bigger, and a 320pt column cannot.
        .overlay {
            if let img = model.detail {
                Lightbox(model: model, img: img)
            }
        }
        .animation(.easeOut(duration: 0.18), value: model.detail?.id)
    }

    /// A page sized to the window rather than the other way round.
    ///
    /// `GeometryReader` sits outside the `ScrollView` on purpose — a scroll view
    /// proposes unbounded height to its content, so measuring from inside would
    /// report infinity and the solver would hand back one enormous row.
    ///
    /// The `ScrollView` stays even when everything fits: nothing scrolls when
    /// the content is exactly as tall as the port, and it is what catches the
    /// small-window case where the solver gives up and falls back to `minCell`.
    private var grid: some View {
        GeometryReader { geo in
            let fit = GridFit.solve(count: model.images.count, in: geo.size,
                                    spacing: Self.spacing, minCell: 72)
            ScrollView {
                LazyVGrid(
                    columns: Array(repeating: GridItem(.fixed(fit.cellWidth),
                                                       spacing: Self.spacing),
                                   count: fit.columns),
                    spacing: Self.spacing
                ) {
                    ForEach(model.images) {
                        Card(model: model, img: $0,
                             width: fit.cellWidth, height: fit.cellHeight)
                    }
                }
                .frame(maxWidth: .infinity)
            }
            .scrollDisabled(!fit.scrolls)
        }
        .padding(.horizontal, 18)
        .padding(.bottom, 14)
    }

    /// The capsule row from the reference. It doubles as the selection bar:
    /// once something is picked the same strip carries the actions, so the
    /// layout does not jump when a checkbox is ticked.
    private var filters: some View {
        HStack(spacing: 10) {
            if model.selection.isEmpty {
                PillRow(items: SortKey.allCases, label: \.label,
                        icon: { $0.icon }, selection: sortBinding)
            } else {
                HStack(spacing: 8) {
                    Text("已选 \(model.selection.count) 张")
                        .font(.callout).foregroundStyle(.white)
                    Button("删除") { Task { await model.deleteSelected() } }
                        .buttonStyle(DangerButton())
                    Button("取消") { model.selection = [] }
                        .buttonStyle(QuietButton())
                }
                .padding(.leading, 4)
            }

            Spacer()

            Button(model.selection.count == model.images.count ? "取消全选" : "全选本页") {
                model.toggleAll()
            }
            .buttonStyle(QuietButton())
        }
        .padding(.horizontal, 18)
        .padding(.bottom, 12)
        .animation(.easeInOut(duration: 0.16), value: model.selection.isEmpty)
    }

    private var sortBinding: Binding<SortKey> {
        Binding(get: { model.sort },
                set: { model.sort = $0; Task { await model.load(resetPage: true) } })
    }

    private var statusBar: some View {
        HStack(spacing: 8) {
            Text("\(model.total) 张")
            if let q = model.quota {
                Text("·").foregroundStyle(.quaternary)
                Text("已用 \(model.bytes(q.usedBytes))")
            }
            Spacer()
            if model.pageCount > 1 {
                ToolCluster {
                    ToolTile(icon: "chevron.left", help: "上一页",
                             disabled: model.page == 0 || model.busy) {
                        Task { await model.go(to: model.page - 1) }
                    }
                    Text("\(model.page + 1) / \(model.pageCount)")
                        .font(.caption.monospacedDigit())
                        .foregroundStyle(.secondary)
                        .frame(minWidth: 46)
                    ToolTile(icon: "chevron.right", help: "下一页",
                             disabled: model.page >= model.pageCount - 1 || model.busy) {
                        Task { await model.go(to: model.page + 1) }
                    }
                }
            }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 18)
        .padding(.vertical, 10)
    }
}

// MARK: - Card

/// A contact-sheet tile: the picture is the whole cell.
///
/// The old card carried a permanent caption strip under the thumbnail. At the
/// tile sizes a full page needs, that strip costs about a third of the cell to
/// print a filename too narrow to read — so the name moved onto the picture,
/// where it only appears under the pointer, and onto the tooltip, which stays
/// legible no matter how small the tile gets.
private struct Card: View {
    @ObservedObject var model: AppModel
    let img: RemoteImage
    let width: Double
    let height: Double
    @State private var hovering = false

    private var selected: Bool { model.selection.contains(img.id) }
    private var active: Bool { model.detail?.id == img.id }
    /// Below this the caption is a few truncated characters and a format chip
    /// wider than the word it holds. The tooltip carries the name instead.
    private var showsCaption: Bool { width >= 118 }

    var body: some View {
        ZStack(alignment: .topLeading) {
            Thumbnail(url: img.thumbURL, client: try? model.client())

            if hovering && showsCaption {
                caption
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
                    .transition(.opacity)
            }

            if hovering || selected || !model.selection.isEmpty {
                Button { model.toggle(img.id) } label: {
                    Image(systemName: selected ? "checkmark.circle.fill" : "circle")
                        .font(.system(size: 15))
                        .symbolRenderingMode(.palette)
                        .foregroundStyle(.white,
                                         selected ? Color.brand : .black.opacity(0.45))
                        .shadow(color: .black.opacity(0.35), radius: 2)
                }
                .buttonStyle(.plain)
                .padding(6)
                .transition(.opacity)
            }
        }
        .frame(width: width, height: height)
        .panelSurface(10)
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .strokeBorder(selected || active ? Color.brand : .clear, lineWidth: 2)
        )
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .animation(.easeOut(duration: 0.12), value: hovering)
        .animation(.easeOut(duration: 0.12), value: selected)
        .hoverLift()
        .onHover { hovering = $0 }
        .help("\(img.origName) · \(img.ext.uppercased())")
        .contentShape(Rectangle())
        .onTapGesture { model.detail = active ? nil : img }
        .contextMenu {
            ForEach(LinkFormat.allCases, id: \.self) { f in
                Button("复制\(f.label)") {
                    model.copy(f.render(img)); model.announce("已复制\(f.label)")
                }
            }
            Divider()
            Button("在浏览器打开") {
                if let u = URL(string: img.shortURL ?? img.url) { NSWorkspace.shared.open(u) }
            }
            Divider()
            Button("删除", role: .destructive) { Task { await model.delete(img) } }
        }
    }

    /// A flat bar rather than a scrim. A gradient over a photograph reads as
    /// part of the photograph.
    private var caption: some View {
        HStack(spacing: 5) {
            Text(img.origName)
                .font(.caption2).lineLimit(1).truncationMode(.middle)
            Spacer(minLength: 0)
            Text(img.ext.uppercased())
                .font(.system(size: 8.5, weight: .medium))
                .foregroundStyle(.white.opacity(0.75))
        }
        .foregroundStyle(.white)
        .padding(.horizontal, 7)
        .padding(.vertical, 5)
        .background(Color.black.opacity(0.62))
    }
}

// MARK: - Empty state

private struct EmptyState: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 14) {
            if model.busy {
                ProgressView()
            } else if !model.search.isEmpty {
                icon("magnifyingglass")
                Text("没有匹配「\(model.search)」的图片").font(.title3.weight(.medium))
                Button("清除搜索") {
                    model.search = ""
                    Task { await model.load(resetPage: true) }
                }
                .buttonStyle(QuietButton())
            } else {
                icon("photo.on.rectangle.angled")
                Text("图库还是空的").font(.title3.weight(.medium))
                Text("拖一张图片进来，或按 ⌘U 选择文件")
                    .font(.callout).foregroundStyle(.secondary)
                Button("上传第一张") { model.section = .upload }
                    .buttonStyle(BrandButton())
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func icon(_ name: String) -> some View {
        Image(systemName: name)
            .font(.system(size: 40, weight: .light))
            .foregroundStyle(Color.brand.opacity(0.65))
            .frame(width: 84, height: 84)
            .background(Circle().fill(Color.brand.opacity(0.10)))
    }
}
