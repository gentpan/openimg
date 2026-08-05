// swift-tools-version: 6.0
import PackageDescription

// OpenimgKit is deliberately a plain SwiftPM library with no dependencies and
// no Xcode project: it builds from the command line, which is the only way CI
// (and contributors who are not on macOS) can touch it. The apps are thin.
//
// The checks live in an executable rather than a test target because both
// XCTest and swift-testing need a full Xcode install to resolve — with only
// the Command Line Tools present, `swift test` cannot compile at all, so a
// test target would be a set of assertions nobody ever runs. `swift run
// KitCheck` works anywhere Swift does. Add an XCTest target alongside it once
// Xcode is in the picture; this one still runs in CI without it.
let package = Package(
    name: "OpenimgKit",
    platforms: [.macOS(.v13), .iOS(.v16)],
    products: [
        .library(name: "OpenimgKit", targets: ["OpenimgKit"]),
        .executable(name: "KitCheck", targets: ["KitCheck"]),
        .executable(name: "OpenimgMac", targets: ["OpenimgMac"]),
    ],
    targets: [
        .target(name: "OpenimgKit"),
        .executableTarget(name: "KitCheck", dependencies: ["OpenimgKit"]),
        .executableTarget(name: "OpenimgMac", dependencies: ["OpenimgKit"]),
    ]
)
