import SwiftUI
import AppKit
import OpenimgKit

/// In-memory cache for decoded thumbnails.
///
/// AsyncImage would be the one-liner here, and it is the wrong tool: it holds
/// nothing between view identities, so scrolling a grid re-downloads and
/// re-decodes every card that leaves and re-enters the viewport. On a page of
/// 200 that is 200 requests per pass over the same images.
///
/// NSCache rather than a dictionary because it evicts under memory pressure on
/// its own — a 200-image page of 600px thumbnails is real memory, and the app
/// should give it back when the system asks rather than when it feels like it.
@MainActor
final class ThumbnailCache {
    static let shared = ThumbnailCache()
    private let cache = NSCache<NSString, NSImage>()
    private var inFlight: [String: Task<NSImage?, Never>] = [:]

    private init() { cache.countLimit = 400 }

    func image(for url: String, client: OpenimgClient?) async -> NSImage? {
        if let hit = cache.object(forKey: url as NSString) { return hit }
        // Two cards can ask for the same URL in the same frame — dedup so the
        // second one waits on the first request instead of starting another.
        if let running = inFlight[url] { return await running.value }

        let task = Task<NSImage?, Never> { [weak self] in
            guard let client else { return nil }
            guard let data = try? await client.fetchData(url),
                  let image = NSImage(data: data) else { return nil }
            await MainActor.run { self?.cache.setObject(image, forKey: url as NSString) }
            return image
        }
        inFlight[url] = task
        let result = await task.value
        inFlight[url] = nil
        return result
    }
}

/// A thumbnail that keeps its loaded image across scrolls.
struct Thumbnail: View {
    let url: String
    let client: OpenimgClient?
    @State private var image: NSImage?

    var body: some View {
        ZStack {
            if let image {
                Image(nsImage: image)
                    .resizable()
                    .aspectRatio(contentMode: .fill)
            } else {
                Rectangle().fill(Color.secondary.opacity(0.12))
                ProgressView().controlSize(.small)
            }
        }
        .task(id: url) {
            image = await ThumbnailCache.shared.image(for: url, client: client)
        }
    }
}
