import Foundation

/// One file's journey through the upload queue.
struct UploadItem: Identifiable {
    enum State: Equatable {
        case queued, uploading, done
        case failed(String)
    }

    let id = UUID()
    let url: URL
    var state: State = .queued
    var progress: Double = 0
    var deduplicated = false

    var name: String { url.lastPathComponent }
    var size: Int64 {
        (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize).map(Int64.init) ?? 0
    }

    /// How much of this file counts as finished, for the batch bar. A rejected
    /// file counts as complete: it is never going to move again, and leaving it
    /// at zero would park the overall bar short of 100% forever.
    var fraction: Double {
        switch state {
        case .queued: 0
        case .uploading: progress
        case .done, .failed: 1
        }
    }
}
