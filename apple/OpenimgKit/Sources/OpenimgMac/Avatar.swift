import SwiftUI
import OpenimgKit

/// The account picture, falling back to an initial.
///
/// The fallback is not a placeholder for "still loading" — it is the answer for
/// accounts that have no picture, and it has to look deliberate rather than
/// broken. The two states are visually the same size and shape so the sidebar
/// does not reflow when one resolves into the other.
///
/// Avatars are not always on our own origin. An uploaded one is on the CDN;
/// one carried over from Google or GitHub still points at their host, which is
/// why the fetch deliberately sends no credentials.
struct Avatar: View {
    let account: Account
    var size: CGFloat = 26
    let client: OpenimgClient?

    @State private var image: NSImage?

    var body: some View {
        Group {
            if let image {
                Image(nsImage: image)
                    .resizable()
                    .aspectRatio(contentMode: .fill)
            } else {
                Circle()
                    .fill(Color.brand.opacity(0.18))
                    .overlay {
                        Text(account.initial)
                            .font(.system(size: size * 0.44, weight: .semibold))
                            .foregroundStyle(Color.brand)
                    }
            }
        }
        .frame(width: size, height: size)
        .clipShape(Circle())
        .overlay(Circle().strokeBorder(.primary.opacity(0.08), lineWidth: 0.5))
        .task(id: account.avatarURL) {
            guard let url = account.avatarURL, !url.isEmpty else { image = nil; return }
            image = await ThumbnailCache.shared.image(for: url, client: client)
        }
    }
}
