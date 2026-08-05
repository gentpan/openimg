import SwiftUI
import AppKit
import OpenimgKit

/// Full-window viewer: the picture large on the left, everything you might do
/// with it on the right.
///
/// This replaces an inspector column that showed a 150pt thumbnail — which is
/// the same size the grid already showed, so opening it told you nothing you
/// could not already see. Clicking a picture should show you the picture.
struct Lightbox: View {
    @ObservedObject var model: AppModel
    let img: RemoteImage
    @State private var copied: String?
    @State private var full: NSImage?
    @State private var loadingFull = false

    var body: some View {
        HStack(spacing: 0) {
            preview
            Divider().overlay(Color.white.opacity(0.08))
            info.frame(width: 320)
        }
        .background(.black.opacity(0.55))
        .background(.ultraThinMaterial)
        .transition(.opacity)
        // Escape closes, like every other viewer on the platform.
        .onExitCommand { model.detail = nil }
    }

    // MARK: - Preview

    private var preview: some View {
        ZStack {
            Color.black.opacity(0.25)

            if let full {
                Image(nsImage: full)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .padding(24)
            } else {
                // The thumbnail stands in while the full image downloads, so
                // the panel opens with something on screen instead of a blank
                // rectangle and a spinner.
                Thumbnail(url: img.thumbURL, client: try? model.client())
                    .aspectRatio(CGFloat(img.width) / CGFloat(max(1, img.height)),
                                 contentMode: .fit)
                    .blur(radius: 6)
                    .padding(24)
                    .overlay { if loadingFull { ProgressView() } }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .contentShape(Rectangle())
        // Clicking the picture closes, the way every viewer on the platform
        // behaves. The arrows sit above this and take their own taps, so
        // paging never falls through to a dismiss.
        .onTapGesture { model.detail = nil }
        .overlay(alignment: .topLeading) {
            Button { model.detail = nil } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 12, weight: .semibold))
                    .frame(width: 28, height: 28)
            }
            .buttonStyle(.plain)
            .background(Circle().fill(.black.opacity(0.45)))
            .padding(14)
            .keyboardShortcut(.escape, modifiers: [])
        }
        .overlay(alignment: .leading) {
            arrow("chevron.left", enabled: model.canGoPrev) { await model.showPrev() }
                .keyboardShortcut(.leftArrow, modifiers: [])
        }
        .overlay(alignment: .trailing) {
            arrow("chevron.right", enabled: model.canGoNext) { await model.showNext() }
                .keyboardShortcut(.rightArrow, modifiers: [])
        }
        .overlay(alignment: .bottom) {
            if let i = model.detailIndex {
                Text("\(model.page * model.pageSize + i + 1) / \(model.total)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 10).padding(.vertical, 5)
                    .background(Capsule().fill(.black.opacity(0.45)))
                    .padding(.bottom, 14)
            }
        }
        .task(id: img.id) {
            full = nil
            loadingFull = true
            defer { loadingFull = false }
            // The original, not the 600px derivative. This used to fetch the
            // derivative on the grounds that the pane is only about a thousand
            // points wide — which ignored that a point is two pixels on every
            // display this app runs on, so 600 was upscaled to 2000 and looked
            // it. Enlarging a picture is the one place that should show what
            // was actually uploaded.
            full = await ThumbnailCache.full.image(for: img.url, client: try? model.client())
        }
    }

    private func arrow(_ icon: String, enabled: Bool,
                       _ action: @escaping () async -> Void) -> some View {
        Button { Task { await action() } } label: {
            Image(systemName: icon)
                .font(.system(size: 15, weight: .semibold))
                .frame(width: 38, height: 38)
                .background(Circle().fill(.black.opacity(0.45)))
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
        .opacity(enabled ? 1 : 0.25)
        .padding(.horizontal, 14)
    }

    // MARK: - Info

    private var info: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                VStack(alignment: .leading, spacing: 5) {
                    Text(img.origName)
                        .font(.headline)
                        .textSelection(.enabled)
                        .fixedSize(horizontal: false, vertical: true)
                    Text(img.createdAt.formatted(date: .abbreviated, time: .shortened))
                        .font(.caption).foregroundStyle(.secondary)
                }

                HStack(spacing: 8) {
                    chip("\(img.width) × \(img.height)", "aspectratio")
                    chip(img.ext.uppercased(), "doc")
                    chip(model.bytes(img.sizeStored), "internaldrive")
                }

                Divider().overlay(Color.white.opacity(0.08))

                Text("复制链接").font(.caption).foregroundStyle(.secondary)
                VStack(spacing: 6) {
                    ForEach(LinkFormat.allCases) { f in
                        copyRow(f)
                    }
                }

                Divider().overlay(Color.white.opacity(0.08))

                VStack(spacing: 8) {
                    if let short = img.shortURL {
                        Button {
                            if let u = URL(string: short) { NSWorkspace.shared.open(u) }
                        } label: {
                            Label("打开分享页", systemImage: "safari").frame(maxWidth: .infinity)
                        }
                        .buttonStyle(QuietButton())
                    }
                    Button {
                        Task { await model.delete(img) }
                    } label: {
                        Label("删除这张图片", systemImage: "trash").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(DangerButton())
                }
            }
            .padding(18)
        }
        .background(.black.opacity(0.2))
    }

    private func chip(_ text: String, _ icon: String) -> some View {
        HStack(spacing: 4) {
            Image(systemName: icon).font(.system(size: 9))
            Text(text)
        }
        .font(.caption)
        .foregroundStyle(.secondary)
        .padding(.horizontal, 8).padding(.vertical, 4)
        .background(Capsule().fill(.white.opacity(0.07)))
    }

    private func copyRow(_ f: LinkFormat) -> some View {
        Button {
            model.copy(f.render(img))
            withAnimation(.easeOut(duration: 0.15)) { copied = f.label }
            Task {
                try? await Task.sleep(for: .seconds(1.6))
                withAnimation(.easeOut(duration: 0.2)) { copied = nil }
            }
        } label: {
            HStack(spacing: 8) {
                Text(f.label)
                    .font(.caption.weight(.medium))
                    .frame(width: 62, alignment: .leading)
                Text(f.render(img))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1).truncationMode(.middle)
                Spacer(minLength: 0)
                Image(systemName: copied == f.label ? "checkmark" : "doc.on.doc")
                    .font(.system(size: 11))
                    .foregroundStyle(copied == f.label ? AnyShapeStyle(Color.brand) : AnyShapeStyle(.tertiary))
            }
            .padding(.horizontal, 10).padding(.vertical, 8)
            .background(
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .fill(.white.opacity(copied == f.label ? 0.10 : 0.05))
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}
