import SwiftUI
import AppKit
import OpenimgKit

struct GalleryView: View {
    @ObservedObject var model: AppModel
    private let columns = [GridItem(.adaptive(minimum: 158, maximum: 210), spacing: 14)]

    var body: some View {
        VStack(spacing: 0) {
            if !model.images.isEmpty { filters }

            if model.images.isEmpty {
                EmptyState(model: model)
            } else {
                ScrollView {
                    LazyVGrid(columns: columns, spacing: 14) {
                        ForEach(model.images) { Card(model: model, img: $0) }
                    }
                    .padding(.horizontal, 18)
                    .padding(.bottom, 18)
                }
            }
            statusBar
        }
        .inspector(isPresented: Binding(
            get: { model.detail != nil },
            set: { if !$0 { model.detail = nil } }
        )) {
            if let img = model.detail {
                DetailView(model: model, img: img)
                    .inspectorColumnWidth(min: 280, ideal: 320, max: 420)
            }
        }
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

private struct Card: View {
    @ObservedObject var model: AppModel
    let img: RemoteImage
    @State private var hovering = false

    private var selected: Bool { model.selection.contains(img.id) }
    private var active: Bool { model.detail?.id == img.id }

    var body: some View {
        VStack(spacing: 0) {
            ZStack(alignment: .topLeading) {
                Thumbnail(url: img.thumbURL, client: try? model.client())
                    .frame(height: 118)
                    .frame(maxWidth: .infinity)
                    .clipped()

                if hovering || selected || !model.selection.isEmpty {
                    Button { model.toggle(img.id) } label: {
                        Image(systemName: selected ? "checkmark.circle.fill" : "circle")
                            .font(.system(size: 16))
                            .symbolRenderingMode(.palette)
                            .foregroundStyle(.white, selected ? Color.brand : .black.opacity(0.45))
                    }
                    .buttonStyle(.plain)
                    .padding(7)
                    .transition(.opacity)
                }
            }

            HStack(spacing: 5) {
                Text(img.origName)
                    .font(.caption2).lineLimit(1).truncationMode(.middle)
                Spacer(minLength: 0)
                Text(img.ext.uppercased())
                    .font(.system(size: 8.5, weight: .medium))
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 4).padding(.vertical, 1.5)
                    .background(Capsule().fill(.white.opacity(0.10)))
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 7)
        }
        .panelSurface(11)
        .overlay(
            RoundedRectangle(cornerRadius: 11, style: .continuous)
                .strokeBorder(selected || active ? Color.brand : .clear, lineWidth: 1.6)
        )
        .clipShape(RoundedRectangle(cornerRadius: 11, style: .continuous))
        .animation(.easeOut(duration: 0.12), value: hovering)
        .animation(.easeOut(duration: 0.12), value: selected)
        .hoverLift()
        .onHover { hovering = $0 }
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
