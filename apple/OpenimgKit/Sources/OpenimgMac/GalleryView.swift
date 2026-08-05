import SwiftUI
import AppKit
import OpenimgKit

struct GalleryView: View {
    @ObservedObject var model: AppModel

    // Adaptive rather than a fixed five columns: this window is resizable, and
    // stretching five cards across a widened window makes the thumbnails grow
    // past the size they were generated at.
    private let columns = [GridItem(.adaptive(minimum: 150, maximum: 200), spacing: 14)]

    var body: some View {
        Group {
            if model.images.isEmpty {
                EmptyState(model: model)
            } else {
                ScrollView {
                    LazyVGrid(columns: columns, spacing: 14) {
                        ForEach(model.images) { Card(model: model, img: $0) }
                    }
                    .padding(18)
                }
                .background(.background)
            }
        }
        .safeAreaInset(edge: .bottom, spacing: 0) { statusBar }
        .inspector(isPresented: Binding(
            get: { model.detail != nil },
            set: { if !$0 { model.detail = nil } }
        )) {
            if let img = model.detail {
                DetailView(model: model, img: img)
                    .inspectorColumnWidth(min: 270, ideal: 310, max: 420)
            }
        }
        .animation(.easeInOut(duration: 0.2), value: model.selection.isEmpty)
    }

    // MARK: - Status bar

    private var statusBar: some View {
        HStack(spacing: 10) {
            if model.selection.isEmpty {
                Text("\(model.total) 张")
                if let q = model.quota {
                    Text("·").foregroundStyle(.quaternary)
                    Text("已用 \(model.bytes(q.usedBytes))")
                }
            } else {
                Text("已选 \(model.selection.count) 张")
                    .foregroundStyle(Color.brand)
                Button("删除", role: .destructive) { Task { await model.deleteSelected() } }
                    .controlSize(.small)
                Button("取消") { model.selection = [] }
                    .controlSize(.small)
            }

            Spacer()

            Button(model.selection.count == model.images.count ? "取消全选" : "全选本页") {
                model.toggleAll()
            }
            .controlSize(.small)
            .disabled(model.images.isEmpty)

            if model.pageCount > 1 {
                Divider().frame(height: 14)
                Button { Task { await model.go(to: model.page - 1) } } label: {
                    Image(systemName: "chevron.left")
                }
                .disabled(model.page == 0 || model.busy)
                Text("\(model.page + 1) / \(model.pageCount)")
                    .monospacedDigit()
                Button { Task { await model.go(to: model.page + 1) } } label: {
                    Image(systemName: "chevron.right")
                }
                .disabled(model.page >= model.pageCount - 1 || model.busy)
            }
        }
        .buttonStyle(.borderless)
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(.bar)
        .overlay(alignment: .top) { Divider() }
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
        VStack(spacing: 5) {
            ZStack(alignment: .topLeading) {
                Thumbnail(url: img.thumbURL, client: try? model.client())
                    .frame(height: 120)
                    .frame(maxWidth: .infinity)
                    .clipShape(RoundedRectangle(cornerRadius: 9))
                    .overlay {
                        RoundedRectangle(cornerRadius: 9)
                            .strokeBorder(
                                selected || active ? Color.brand : Color.primary.opacity(0.10),
                                lineWidth: selected || active ? 2 : 1
                            )
                    }

                // The checkbox only appears on hover or when something is
                // already selected. A permanent one on every tile turns a
                // gallery into a form.
                if hovering || selected || !model.selection.isEmpty {
                    Button {
                        model.toggle(img.id)
                    } label: {
                        Image(systemName: selected ? "checkmark.circle.fill" : "circle")
                            .font(.system(size: 16))
                            .symbolRenderingMode(.palette)
                            .foregroundStyle(.white, selected ? Color.brand : .black.opacity(0.35))
                    }
                    .buttonStyle(.plain)
                    .padding(6)
                    .transition(.opacity)
                }
            }

            Text(img.origName)
                .font(.caption2)
                .lineLimit(1)
                .truncationMode(.middle)
                .foregroundStyle(active ? Color.brand : .secondary)
        }
        .animation(.easeOut(duration: 0.12), value: hovering)
        .animation(.easeOut(duration: 0.12), value: selected)
        .hoverLift()
        .onHover { hovering = $0 }
        .contentShape(Rectangle())
        .onTapGesture { model.detail = active ? nil : img }
        .contextMenu {
            ForEach(LinkFormat.allCases, id: \.self) { f in
                Button("复制\(f.label)") { model.copy(f.render(img)); model.announce("已复制\(f.label)") }
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
        VStack(spacing: 12) {
            if model.busy {
                ProgressView()
            } else if !model.connected {
                icon("link.badge.plus")
                Text("还没有连接").font(.title3.weight(.medium))
                Text("在设置里填入服务器地址和访问令牌").foregroundStyle(.secondary)
                Button("去设置") { model.section = .settings }
                    .buttonStyle(.borderedProminent)
            } else if !model.search.isEmpty {
                icon("magnifyingglass")
                Text("没有匹配「\(model.search)」的图片").font(.title3.weight(.medium))
                Button("清除搜索") {
                    model.search = ""
                    Task { await model.load(resetPage: true) }
                }
            } else {
                icon("photo.on.rectangle.angled")
                Text("图库还是空的").font(.title3.weight(.medium))
                Text("拖一张图片进来，或按 ⌘U 选择文件").foregroundStyle(.secondary)
                Button("上传第一张") { model.section = .upload }
                    .buttonStyle(.borderedProminent)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(.background)
    }

    private func icon(_ name: String) -> some View {
        Image(systemName: name)
            .font(.system(size: 42, weight: .light))
            .foregroundStyle(Color.brand.opacity(0.5))
            .padding(.bottom, 2)
    }
}
