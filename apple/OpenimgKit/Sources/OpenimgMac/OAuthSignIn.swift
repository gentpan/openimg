import AppKit
import AuthenticationServices
import OpenimgKit

/// Google / GitHub sign-in through the system's web authentication sheet.
///
/// `ASWebAuthenticationSession` rather than opening the default browser: the
/// sheet is scoped to this app, tells us when it finished, and — importantly —
/// does not leave the user logged into the site in a browser they did not ask
/// to open. The cost is that the flow must end at a URL scheme we own, which
/// is why the server takes `?native=1` and redirects to `openimg://auth?code=`
/// instead of its usual redirect to `/`.
///
/// What comes back is a single-use code, not a credential. It expires in a
/// minute and is consumed on first use, so the value that crosses the process
/// boundary is worthless to anyone who intercepts it after the fact.
@MainActor
final class OAuthSignIn: NSObject, ASWebAuthenticationPresentationContextProviding {
    private var session: ASWebAuthenticationSession?

    func presentationAnchor(for _: ASWebAuthenticationSession) -> ASPresentationAnchor {
        NSApp.keyWindow ?? NSApp.windows.first ?? ASPresentationAnchor()
    }

    /// Returns the one-time code, or nil when the user closed the sheet.
    func start(provider: String, server: String) async throws -> String? {
        guard let url = URL(string: "\(server)/auth/\(provider)/start?native=1") else {
            throw OpenimgError.badServerURL
        }

        return try await withCheckedThrowingContinuation { cont in
            let s = ASWebAuthenticationSession(
                url: url, callbackURLScheme: OpenimgAuth.nativeScheme
            ) { callback, error in
                if let error = error as? ASWebAuthenticationSessionError,
                   error.code == .canceledLogin {
                    cont.resume(returning: nil) // 用户关掉了窗口，不是错误
                    return
                }
                if let error { cont.resume(throwing: error); return }
                guard let callback,
                      let code = URLComponents(url: callback, resolvingAgainstBaseURL: false)?
                          .queryItems?.first(where: { $0.name == "code" })?.value
                else {
                    cont.resume(throwing: OpenimgError.transport("登录回调里没有拿到登录码"))
                    return
                }
                cont.resume(returning: code)
            }
            s.presentationContextProvider = self
            // A shared cookie jar would silently reuse whichever account the
            // user last signed into on this Mac, which is the wrong default
            // for a login button and impossible to recover from in the sheet.
            s.prefersEphemeralWebBrowserSession = true
            session = s
            if !s.start() {
                cont.resume(throwing: OpenimgError.transport("无法打开登录窗口"))
            }
        }
    }
}
