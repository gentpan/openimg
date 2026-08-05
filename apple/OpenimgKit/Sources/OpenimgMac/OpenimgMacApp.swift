import SwiftUI
import AppKit
import OpenimgKit

@main
struct OpenimgMacApp: App {
    @StateObject private var model = AppModel.shared

    var body: some Scene {
        Window("Openimg", id: "main") {
            RootView(model: model)
                .tint(.brand)
        }
        .defaultSize(width: 1020, height: 680)
        // The title bar carries the toolbar, so drawing a title in it too just
        // repeats what the sidebar already says.
        .windowStyle(.hiddenTitleBar)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("上传图片…") {
                    model.section = .upload
                    Task { await model.pickAndUpload() }
                }
                .keyboardShortcut("u")
                .disabled(!model.connected)
            }
            CommandGroup(after: .toolbar) {
                Button("刷新图库") { Task { await model.load() } }
                    .keyboardShortcut("r")
                    .disabled(!model.connected)
            }
        }
    }
}

struct RootView: View {
    @ObservedObject var model: AppModel
    @State private var columnVisibility = NavigationSplitViewVisibility.all

    var body: some View {
        NavigationSplitView(columnVisibility: $columnVisibility) {
            Sidebar(model: model)
                .navigationSplitViewColumnWidth(min: 180, ideal: 200, max: 240)
        } detail: {
            detail
                .toolbar { toolbar }
        }
        .task { await model.restore() }
        .overlay(alignment: .bottom) { toast }
    }

    @ViewBuilder
    private var detail: some View {
        switch model.section {
        case .overview: OverviewView(model: model)
        case .gallery:  GalleryView(model: model)
        case .upload:   UploadView(model: model)
        case .settings: SettingsView(model: model)
        }
    }

    @ToolbarContentBuilder
    private var toolbar: some ToolbarContent {
        if model.section == .gallery && model.connected {
            ToolbarItem(placement: .navigation) {
                TextField("搜索文件名", text: $model.search)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 190)
                    .onSubmit { Task { await model.load(resetPage: true) } }
                    .onChange(of: model.search) { model.searchChanged() }
            }
            ToolbarItem {
                Picker("排序", selection: $model.sort) {
                    ForEach(SortKey.allCases) { Label($0.label, systemImage: $0.icon).tag($0) }
                }
                .onChange(of: model.sort) { Task { await model.load(resetPage: true) } }
            }
            ToolbarItem {
                Picker("每页", selection: $model.pageSize) {
                    ForEach(model.pageSizes, id: \.self) { Text("\($0) 张/页").tag($0) }
                }
                .onChange(of: model.pageSize) { Task { await model.load(resetPage: true) } }
            }
            ToolbarItem {
                Button {
                    Task { await model.load() }
                } label: {
                    Label("刷新", systemImage: "arrow.clockwise")
                }
                .disabled(model.busy)
            }
        }
        if model.connected {
            ToolbarItem {
                Button {
                    model.section = .upload
                    Task { await model.pickAndUpload() }
                } label: {
                    Label("上传", systemImage: "arrow.up.circle.fill")
                }
            }
        }
    }

    /// Transient feedback, not a permanent bar. A status line that never goes
    /// away becomes furniture and stops being read.
    @ViewBuilder
    private var toast: some View {
        if !model.status.isEmpty {
            Text(model.status)
                .font(.callout)
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .background(.regularMaterial, in: Capsule())
                .overlay(Capsule().strokeBorder(.quaternary))
                .shadow(color: .black.opacity(0.15), radius: 10, y: 4)
                .padding(.bottom, 22)
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .onTapGesture { model.status = "" }
        }
    }
}

struct Sidebar: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(spacing: 0) {
            List(selection: Binding(
                get: { model.section },
                set: { if let s = $0 { model.section = s } }
            )) {
                Section {
                    ForEach(Section_.allCases) { s in
                        Label(s.label, systemImage: s.icon)
                            .tag(s)
                            // Gallery and upload need a connection; settings is
                            // where you go to get one, so it stays reachable.
                            .foregroundStyle(model.connected || s == .settings ? .primary : .tertiary)
                    }
                }
            }
            .listStyle(.sidebar)

            Divider()
            account
        }
    }

    private var account: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let a = model.account {
                HStack(spacing: 8) {
                    Circle()
                        .fill(Color.brand.opacity(0.18))
                        .frame(width: 26, height: 26)
                        .overlay {
                            Text(String((a.name.isEmpty ? a.email : a.name).prefix(1)).uppercased())
                                .font(.caption.weight(.semibold))
                                .foregroundStyle(Color.brand)
                        }
                    VStack(alignment: .leading, spacing: 0) {
                        Text(a.name.isEmpty ? a.email : a.name)
                            .font(.callout)
                            .lineLimit(1)
                        if let q = model.quota {
                            Text("\(model.bytes(q.availableBytes)) 可用")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
                if let q = model.quota, q.quotaBytes > 0 {
                    ProgressView(value: min(1, Double(q.usedBytes) / Double(q.quotaBytes)))
                        .controlSize(.small)
                }
            } else {
                Label("未连接", systemImage: "person.crop.circle.badge.questionmark")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
    }
}
