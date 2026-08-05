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

CONFIG=${1:-debug}
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG="$ROOT/OpenimgKit"
APP="$ROOT/build/OpenimgMac.app"

echo "构建 ($CONFIG)…"
swift build --package-path "$PKG" -c "$CONFIG" --product OpenimgMac >/dev/null
BIN="$(swift build --package-path "$PKG" -c "$CONFIG" --show-bin-path)/OpenimgMac"
[[ -x "$BIN" ]] || { echo "找不到产物：$BIN" >&2; exit 1; }

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS"
cp "$BIN" "$APP/Contents/MacOS/OpenimgMac"

# LSUIElement is the line that makes this a menu bar app rather than a normal
# one: without it the app takes a Dock icon and a menu bar of its own, which is
# wrong for something whose entire UI is one status item.
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
  <key>LSMinimumSystemVersion</key>      <string>13.0</string>
  <key>LSUIElement</key>                 <true/>
  <key>NSHighResolutionCapable</key>     <true/>
</dict>
</plist>
PLIST

# Ad-hoc signature. Not a substitute for a Developer ID — it lets the app run on
# this machine, nothing more — but an unsigned bundle is refused outright on
# Apple silicon, so it is required even locally.
codesign --force --sign - --identifier io.openimg.mac "$APP" 2>/dev/null

echo "已打包：$APP"
codesign -dv "$APP" 2>&1 | grep -E "Identifier|Signature" | sed 's/^/  /'
echo
echo "运行：open '$APP'"
echo "退出：pkill -x OpenimgMac"
