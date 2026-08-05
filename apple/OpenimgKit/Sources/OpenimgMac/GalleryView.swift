import SwiftUI
import OpenimgKit

struct GalleryView: View {
    @ObservedObject var model: AppModel

    // Fixed five columns, matching the web gallery. Every page size divides by
    // five, so a full page never ends in a short row with holes in it.
    private let columns = Array(repeating: GridItem(.flexible(), spacing: 12), count: 5)

    var body: some View {
        VStack(spacing: 0) {
            toolbar
            Divider()

            if model.images.isEmpty {
                empty
            } else {
                ScrollView {
                    LazyVGrid(columns: columns, spacing: 12) {
                        ForEach(model.images) { img in
                            card(img)
                        }
                    }
                    .padding(16)
                }
            }

            Divider()
            pager
        }
        .inspector(isPresented: .constant(model.detail != nil)) {
            if let img = model.detail {
                DetailView(model: model, img: img)
                    .inspectorColumnWidth(min: 260, ideal: 300, max: 400)
            }
        }
    }

    // MARK: - Toolbar

    private var toolbar: some View {
        HStack(spacing: 10) {
            TextField("搜索文件名", text: $model.search)
                .textFieldStyle(.roundedBorder)
                .frame(width: 200)
                .onChange(of: model.search) { model.searchChanged() }

            Picker("", selection: $model.sort) {
                ForEach(SortKey.allCases) { Text($0.label).tag($0) }
            }
            .labelsHidden()
            .frame(width: 130)
            .onChange(of: model.sort) { Task { await model.load(resetPage: true) } }

            Picker("", selection: $model.pageSize) {
                ForEach(model.pageSizes, id: \.self) { Text("\($0)/页").tag($0) }
            }
            .labelsHidden()
            .frame(width: 90)
            .onChange(of: model.pageSize) { Task { await model.load(resetPage: true) } }

            Spacer()

            if !model.selection.isEmpty {
                Text("已选 \(model.selection.count)").foregroundStyle(.secondary)
                Button("删除选中", role: .destructive) {
                    Task { await model.deleteSelected() }
                }
            }
            Button(model.selection.count == model.images.count && !model.images.isEmpty
                   ? "取消全选" : "全选本页") {
                model.toggleAll()
            }
            .disabled(model.images.isEmpty)

            Button {
                Task { await model.load() }
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .disabled(model.busy)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }

    // MARK: - Card

    private func card(_ img: RemoteImage) -> some View {
        let selected = model.selection.contains(img.id)
        return VStack(spacing: 4) {
            ZStack(alignment: .topLeading) {
                Thumbnail(url: img.thumbURL, client: try? model.client())
                    .frame(height: 110)
                    .clipped()
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .overlay {
                        RoundedRectangle(cornerRadius: 8)
                            .strokeBorder(selected ? Color.accentColor : Color.secondary.opacity(0.25),
                                          lineWidth: selected ? 2 : 1)
                    }

                Button {
                    model.toggle(img.id)
                } label: {
                    Image(systemName: selected ? "checkmark.circle.fill" : "circle")
                        .font(.system(size: 15))
                        .foregroundStyle(selected ? Color.accentColor : .white)
                        .shadow(radius: 1)
                        .padding(5)
                }
                .buttonStyle(.plain)
            }
            Text(img.origName)
                .font(.caption2)
                .lineLimit(1)
                .truncationMode(.middle)
                .foregroundStyle(.secondary)
        }
        .contentShape(Rectangle())
        .onTapGesture { model.detail = img }
        .contextMenu {
            ForEach(LinkFormat.allCases, id: \.self) { f in
                Button("复制 \(f.label)") { model.copy(f.render(img)) }
            }
            Divider()
            Button("在浏览器打开") {
                if let u = URL(string: img.shortURL ?? img.url) { NSWorkspace.shared.open(u) }
            }
            Divider()
            Button("删除", role: .destructive) { Task { await model.delete(img) } }
        }
    }

    // MARK: - Empty & pager

    private var empty: some View {
        VStack(spacing: 8) {
            Spacer()
            if model.busy {
                ProgressView()
            } else {
                Image(systemName: "photo.on.rectangle.angled")
                    .font(.system(size: 34))
                    .foregroundStyle(.tertiary)
                Text(model.search.isEmpty ? "图库还是空的" : "没有匹配「\(model.search)」的图片")
                    .foregroundStyle(.secondary)
                if model.search.isEmpty {
                    Button("去上传") { model.tab = .upload }
                }
            }
            Spacer()
        }
        .frame(maxWidth: .infinity)
    }

    private var pager: some View {
        HStack {
            Text("\(model.total) 张")
            if let q = model.quota {
                Text("·").foregroundStyle(.tertiary)
                Text("\(model.bytes(q.usedBytes)) 已用 / \(model.bytes(q.availableBytes)) 可用")
            }
            Spacer()
            if model.pageCount > 1 {
                Button { Task { await model.go(to: model.page - 1) } } label: {
                    Image(systemName: "chevron.left")
                }
                .disabled(model.page == 0 || model.busy)
                Text("\(model.page + 1) / \(model.pageCount)").monospacedDigit()
                Button { Task { await model.go(to: model.page + 1) } } label: {
                    Image(systemName: "chevron.right")
                }
                .disabled(model.page >= model.pageCount - 1 || model.busy)
            }
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
    }
}
