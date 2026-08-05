import Foundation
import Security

/// Keychain-backed storage for the personal access token.
///
/// A token is the whole account as far as the image library goes: it can list,
/// upload, and bulk-delete 500 images per call with no second confirmation. It
/// does not belong in UserDefaults, and it does not belong in a plist next to
/// the app.
///
/// `kSecUseDataProtectionKeychain` is set explicitly because it is the one
/// place iOS and macOS disagree by default. A non-sandboxed macOS process
/// otherwise lands in the older file-based keychain, where the item is
/// exportable with the `security` command line tool and `kSecAttrAccessible`
/// does not mean what it means on iOS. Asking for the data-protection keychain
/// on both gets one behaviour instead of two.
public struct TokenStore: Sendable {
    public let service: String
    public init(service: String = "io.openimg.token") { self.service = service }

    /// One entry per server, so connecting to a self-hosted instance does not
    /// silently overwrite the credential for the public one.
    private func query(server: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: server,
            kSecUseDataProtectionKeychain as String: true,
        ]
    }

    public func save(_ token: String, server: String) throws {
        var attrs = query(server: server)
        SecItemDelete(attrs as CFDictionary) // upsert; add-then-update races
        attrs[kSecValueData as String] = Data(token.utf8)
        attrs[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        let status = SecItemAdd(attrs as CFDictionary, nil)
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    public func load(server: String) -> String? {
        var q = query(server: server)
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess,
              let data = out as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @discardableResult
    public func delete(server: String) -> Bool {
        SecItemDelete(query(server: server) as CFDictionary) == errSecSuccess
    }
}

public struct KeychainError: Error, LocalizedError {
    public let status: OSStatus
    public var errorDescription: String? {
        let detail = SecCopyErrorMessageString(status, nil) as String? ?? "未知错误"
        return "钥匙串操作失败（\(status)）：\(detail)"
    }
}
