import Foundation
import Security

/// Keychain-backed storage for the personal access token.
///
/// A token is the whole account as far as the image library goes: it can list,
/// upload, and bulk-delete 500 images per call with no second confirmation. It
/// does not belong in UserDefaults, and it does not belong in a plist next to
/// the app.
///
/// The data-protection keychain is preferred and not required. It is the one
/// place iOS and macOS disagree by default — a non-sandboxed macOS process
/// otherwise lands in the older file-based keychain, where the item is
/// exportable with the `security` command line tool and `kSecAttrAccessible`
/// does not mean what it means on iOS.
///
/// But asking for it needs a keychain-access-group entitlement, which is tied
/// to a team identifier, which an ad-hoc signature does not have. A locally
/// built app therefore gets errSecMissingEntitlement (-34018) and cannot store
/// anything at all. Since the choice is between the older keychain and no
/// keychain, every operation falls back to the older one.
public struct TokenStore: Sendable {
    public let service: String
    public init(service: String = "io.openimg.token") { self.service = service }

    /// One entry per server, so connecting to a self-hosted instance does not
    /// silently overwrite the credential for the public one.
    private func query(server: String, dataProtection: Bool) -> [String: Any] {
        var q: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: server,
        ]
        if dataProtection { q[kSecUseDataProtectionKeychain as String] = true }
        return q
    }

    /// Runs `body` against the data-protection keychain, retrying on the older
    /// one when this build is not entitled to the former.
    private func withKeychain(_ body: (Bool) -> OSStatus) -> OSStatus {
        let status = body(true)
        if status == errSecMissingEntitlement { return body(false) }
        return status
    }

    public func save(_ token: String, server: String) throws {
        let status = withKeychain { dp in
            var attrs = query(server: server, dataProtection: dp)
            SecItemDelete(attrs as CFDictionary) // upsert; add-then-update races
            attrs[kSecValueData as String] = Data(token.utf8)
            attrs[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
            return SecItemAdd(attrs as CFDictionary, nil)
        }
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    public func load(server: String) -> String? {
        var found: Data?
        _ = withKeychain { dp in
            var q = query(server: server, dataProtection: dp)
            q[kSecReturnData as String] = true
            q[kSecMatchLimit as String] = kSecMatchLimitOne
            var out: CFTypeRef?
            let st = SecItemCopyMatching(q as CFDictionary, &out)
            if st == errSecSuccess { found = out as? Data }
            return st
        }
        guard let data = found else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @discardableResult
    public func delete(server: String) -> Bool {
        // Both keychains, not just whichever answers first: a build that
        // gained entitlements mid-life would otherwise leave the old copy.
        var removed = false
        for dp in [true, false] {
            if SecItemDelete(query(server: server, dataProtection: dp) as CFDictionary) == errSecSuccess {
                removed = true
            }
        }
        return removed
    }
}

public struct KeychainError: Error, LocalizedError {
    public let status: OSStatus
    public var errorDescription: String? {
        let detail = SecCopyErrorMessageString(status, nil) as String? ?? "未知错误"
        return "钥匙串操作失败（\(status)）：\(detail)"
    }
    /// True when the app simply is not allowed to use the keychain, as opposed
    /// to the keychain refusing this particular item. Callers can carry on
    /// without persistence instead of treating it as a hard failure.
    public var isEntitlementProblem: Bool { status == errSecMissingEntitlement }
}
