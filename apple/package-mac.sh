#!/usr/bin/env bash
#
# Wraps the OpenimgMac executable in a real .app bundle.
#
# A bare SwiftPM executable does not work for this app, and the failure is
# quiet: MenuBarExtra has nowhere to install a status item without a bundle, so
# the process starts, finds no scene to keep alive, and exits 0. No crash, no
# log, no icon — it just looks like nothing happened.
#
# None of this needs Xcode. A .app is a directory with an Info.plist, and
# codesign ships with the Command Line Tools. Xcode only enters the picture for
# a Developer ID identity and notarization, i.e. giving the app to someone else.
#
#   ./apple/package-mac.sh              debug 构建
#   ./apple/package-mac.sh release      release 构建
set -euo pipefail

# SwiftUI's property wrappers are macros now, and their plugin ships only with
# Xcode — the Command Line Tools toolchain cannot expand @State at all. So the
# build has to run against Xcode's toolchain even when xcode-select points at
# the CLT, which is the default on a machine that had them first.
if [[ -z "${DEVELOPER_DIR:-}" ]]; then
    for candidate in /Applications/Xcode.app /Applications/Xcode-beta.app; do
        if [[ -d "$candidate/Contents/Developer/Toolchains" ]]; then
            export DEVELOPER_DIR="$candidate/Contents/Developer"
            break
        fi
    done
fi
[[ -n "${DEVELOPER_DIR:-}" ]] || { echo "找不到 Xcode —— SwiftUI 宏无法展开" >&2; exit 1; }

CONFIG=${1:-debug}
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG="$ROOT/OpenimgKit"
APP="$ROOT/build/OpenimgMac.app"

echo "构建 ($CONFIG)…"
xcrun swift build --package-path "$PKG" -c "$CONFIG" --product OpenimgMac >/dev/null
BIN="$(xcrun swift build --package-path "$PKG" -c "$CONFIG" --show-bin-path)/OpenimgMac"
[[ -x "$BIN" ]] || { echo "找不到产物：$BIN" >&2; exit 1; }

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/OpenimgMac"
cp "$ROOT/Resources/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
# Ubuntu, registered at launch by BrandFont. Flat in Resources because
# Bundle.main.url(forResource:) does not search subdirectories.
cp "$ROOT"/Resources/Fonts/*.ttf "$APP/Contents/Resources/"
cp "$ROOT/Resources/Fonts/LICENSE-Ubuntu.txt" "$APP/Contents/Resources/"

# No LSUIElement: this is a windowed app, so it should take a Dock icon and its
# own menu bar like any other. The earlier menu-bar-only build set it, and
# leaving it in would give a main-window app no way to be brought back once its
# window is closed.
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>          <string>OpenimgMac</string>
  <key>CFBundleIdentifier</key>          <string>io.openimg.mac</string>
  <key>CFBundleName</key>                <string>Openimg</string>
  <key>CFBundleDisplayName</key>         <string>Openimg</string>
  <key>CFBundlePackageType</key>         <string>APPL</string>
  <key>CFBundleShortVersionString</key>  <string>0.1.0</string>
  <key>CFBundleVersion</key>             <string>1</string>
  <key>LSMinimumSystemVersion</key>      <string>14.0</string>
  <key>CFBundleIconFile</key>            <string>AppIcon</string>
  <!-- The OAuth callback lands here. Without this the system has nothing to
       hand openimg://auth?code=… to, and the sign-in sheet just sits there. -->
  <key>CFBundleURLTypes</key>
  <array>
    <dict>
      <key>CFBundleURLName</key>       <string>io.openimg.mac.auth</string>
      <key>CFBundleURLSchemes</key>    <array><string>openimg</string></array>
    </dict>
  </array>
  <key>NSHighResolutionCapable</key>     <true/>
</dict>
</plist>
PLIST

# Signing.
#
# Ad-hoc by default: it lets the app run on this machine and nothing more, but
# an unsigned bundle is refused outright on Apple silicon, so it is required
# even locally.
#
# Set SIGN_IDENTITY to a real certificate ("Developer ID Application: Name
# (TEAMID)") to get an application identifier, which is what the passkey and
# data-protection-keychain entitlements are checked against. Ad-hoc has no team
# id, so entitlements embedded into an ad-hoc build are ignored — the passkey
# API answers ASAuthorizationError 1004 and the keychain answers -34018.
#
#   SIGN_IDENTITY="Developer ID Application: 你的名字 (ABCDE12345)" ./package-mac.sh
#
# List what is available with: security find-identity -v -p codesigning
ENTITLEMENTS="$ROOT/Resources/OpenimgMac.entitlements"
if [ -n "${SIGN_IDENTITY:-}" ]; then
  codesign --force --options runtime --timestamp \
           --sign "$SIGN_IDENTITY" \
           --entitlements "$ENTITLEMENTS" \
           --identifier io.openimg.mac "$APP"
else
  # No --entitlements here on purpose: ad-hoc cannot honour them, and embedding
  # ones that silently do nothing makes the build look configured when it is not.
  codesign --force --sign - --identifier io.openimg.mac "$APP" 2>/dev/null
fi

echo "已打包：$APP"
codesign -dv "$APP" 2>&1 | grep -E "Identifier|Signature|TeamIdentifier" | sed 's/^/  /'
echo
echo "运行：open '$APP'"
echo "退出：pkill -x OpenimgMac"
