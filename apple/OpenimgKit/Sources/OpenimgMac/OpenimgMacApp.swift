import SwiftUI
import AppKit
import OpenimgKit

@main
struct OpenimgMacApp: App {
    @StateObject private var model = AppModel.shared

    init() { BrandFont.register() }

    var body: some Scene {
        Window("Openimg", id: "main") {
            RootView(model: model)
                .tint(.brand)
                // Dark regardless of the system setting. See Theme.swift: the
                // translucency this design is built on only reads as glass over
                // a dark base, and following the appearance would mean carrying
                // two sets of every surface value.
                .preferredColorScheme(.dark)
        }
        .defaultSize(width: 1060, height: 700)
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
                Button("刷新") { Task { await model.refreshCurrent() } }
                    .keyboardShortcut("r")
                    .disabled(!model.connected)
            }
        }
    }
}

struct RootView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        ZStack(alignment: .bottom) {
            if model.connected {
                HStack(spacing: 0) {
                    Sidebar(model: model).frame(width: 224)
                    content
                }
            } else {
                LoginView(model: model)
            }
            toast
        }
        .frame(minWidth: 900, minHeight: 560)
        .windowSurface()
        .task { await model.restore() }
    }

    private var content: some View {
        VStack(spacing: 0) {
            TopBar(model: model)
            Group {
                switch model.section {
                case .overview: OverviewView(model: model)
                case .gallery:  GalleryView(model: model)
                case .upload:   UploadView(model: model)
                case .settings: SettingsView(model: model)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .contentSurface()
        // Its own rounded plane, so the two columns separate by surface rather
        // than by a hairline.
        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .strokeBorder(.white.opacity(0.07), lineWidth: 0.8)
        )
        .padding(.vertical, 8)
        .padding(.trailing, 8)
    }

    @ViewBuilder
    private var toast: some View {
        if !model.status.isEmpty {
            Text(model.status)
                .font(.callout)
                .foregroundStyle(.white)
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .chromeSurface(Capsule())
                .shadow(color: .black.opacity(0.4), radius: 14, y: 5)
                .padding(.bottom, 26)
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .onTapGesture { model.status = "" }
        }
    }
}

// MARK: - Top bar

/// Hand-built rather than a `.toolbar`.
///
/// The reference groups controls into floating clusters with gaps between them;
/// a system toolbar lays its items on one continuous strip, so the pieces exist
/// but the shape does not. Building it here also lets the bar change with the
/// section instead of every control living on every page.
struct TopBar: View {
    @ObservedObject var model: AppModel

    var body: some View {
        HStack(spacing: 10) {
            Text(model.section.label)
                .font(.title3.weight(.semibold))

            Spacer(minLength: 12)

            if model.section == .gallery {
                search
                ToolCluster {
                    menuTile("arrow.up.arrow.down", "排序") {
                        ForEach(SortKey.allCases) { s in
                            Button {
                                model.sort = s
                                Task { await model.load(resetPage: true) }
                            } label: {
                                Label(s.label, systemImage: model.sort == s ? "checkmark" : s.icon)
                            }
                        }
                    }
                    menuTile("square.grid.2x2", "每页数量") {
                        ForEach(model.pageSizes, id: \.self) { n in
                            Button {
                                model.pageSize = n
                                Task { await model.load(resetPage: true) }
                            } label: {
                                Label("\(n) 张/页",
                                      systemImage: model.pageSize == n ? "checkmark" : "square.grid.2x2")
                            }
                        }
                    }
                }
            }

            ToolCluster {
                ToolTile(icon: "arrow.clockwise", help: "刷新", disabled: model.busy) {
                    Task { await model.refreshCurrent() }
                }
                ToolTile(icon: "arrow.up.circle", help: "上传 (⌘U)") {
                    model.section = .upload
                    Task { await model.pickAndUpload() }
                }
            }
        }
        .padding(.horizontal, 18)
        .padding(.top, 14)
        .padding(.bottom, 12)
    }

    private var search: some View {
        HStack(spacing: 6) {
            Image(systemName: "magnifyingglass")
                .font(.system(size: 11)).foregroundStyle(.tertiary)
            TextField("搜索文件名", text: $model.search)
                .textFieldStyle(.plain)
                .font(.callout)
                .onChange(of: model.search) { model.searchChanged() }
            if !model.search.isEmpty {
                Button {
                    model.search = ""
                    Task { await model.load(resetPage: true) }
                } label: {
                    Image(systemName: "xmark.circle.fill").font(.system(size: 11))
                }
                .buttonStyle(.plain).foregroundStyle(.tertiary)
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .frame(width: 210)
        .chromeSurface(Capsule(), elevated: false)
    }

    /// A tile that opens a menu, keeping the tile's own look instead of the
    /// system menu button's chrome.
    private func menuTile<C: View>(_ icon: String, _ help: String,
                                   @ViewBuilder items: () -> C) -> some View {
        Menu {
            items()
        } label: {
            Image(systemName: icon)
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(width: 30, height: 26)
                .contentShape(Rectangle())
        }
        .menuStyle(.borderlessButton)
        .menuIndicator(.hidden)
        .fixedSize()
        .help(help)
    }
}

// MARK: - Sidebar

/// Hand-drawn instead of `List(.sidebar)`.
///
/// The system list brings its own selection shape, insets and hover behaviour,
/// none of which match the reference's rounded highlight — and overriding them
/// costs more than drawing four rows.
struct Sidebar: View {
    @ObservedObject var model: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack(spacing: 0) {
                Text("Open").font(.brand(17, .bold))
                Text("img").font(.brand(17, .bold)).foregroundStyle(Color.brand)
            }
            .padding(.horizontal, 18)
            .padding(.bottom, 18)

            VStack(spacing: 4) {
                ForEach(Section_.allCases) { s in
                    SidebarRow(
                        section: s,
                        active: model.section == s,
                        busy: s == .upload && model.uploading
                    ) { model.section = s }
                }
            }
            .padding(.horizontal, 10)

            Spacer()
            account
        }
        .padding(.top, 42) // clears the traffic lights
    }

    @ViewBuilder
    private var account: some View {
        if let a = model.account {
            VStack(alignment: .leading, spacing: 8) {
                HStack(spacing: 9) {
                    Avatar(account: a, size: 30, client: try? model.client())
                    VStack(alignment: .leading, spacing: 1) {
                        Text(a.name.isEmpty ? a.email : a.name)
                            .font(.callout).lineLimit(1)
                        if let q = model.quota {
                            Text("\(model.bytes(q.availableBytes)) 可用")
                                .font(.caption2).foregroundStyle(.secondary)
                        }
                    }
                }
                if let q = model.quota, q.quotaBytes > 0 {
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            Capsule().fill(.white.opacity(0.10))
                            Capsule().fill(Color.brand)
                                .frame(width: max(3, geo.size.width *
                                    min(1, Double(q.usedBytes) / Double(q.quotaBytes))))
                        }
                    }
                    .frame(height: 4)
                }
            }
            .padding(12)
            .panelSurface(12)
            .padding(.horizontal, 10)
            .padding(.bottom, 12)
        }
    }
}

private struct SidebarRow: View {
    let section: Section_
    let active: Bool
    /// Upload keeps working while the user browses elsewhere, so the row is
    /// where that shows — a spinner buried on the upload page tells nobody
    /// anything once they have navigated away.
    var busy = false
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 11) {
                Image(systemName: active ? section.iconFilled : section.icon)
                    .font(.system(size: 16, weight: .medium))
                    .frame(width: 22)
                    // Outline morphs into solid on selection.
                    .contentTransition(.symbolEffect(.replace.downUp))
                    // A small bounce when it becomes the active one. Tied to
                    // `active` rather than fired on tap so re-tapping the
                    // current row does not jiggle for no reason.
                    .symbolEffect(.bounce.up.byLayer, value: active)
                    // And a slow pulse while an upload is running.
                    .symbolEffect(.pulse, isActive: busy)
                Text(section.label).font(.system(size: 15))
                Spacer(minLength: 0)
            }
            .foregroundStyle(active ? .white : .secondary)
            .padding(.horizontal, 12)
            .padding(.vertical, 10)
            .background {
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(active ? AnyShapeStyle(Color.brand.opacity(0.85))
                                 : AnyShapeStyle(Color.white.opacity(hovering ? 0.07 : 0)))
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .animation(.easeOut(duration: 0.12), value: active)
        .animation(.easeOut(duration: 0.12), value: hovering)
    }
}
