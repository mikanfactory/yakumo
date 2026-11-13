// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Yakumo",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(url: "https://github.com/sindresorhus/KeyboardShortcuts", "1.10.0" ..< "1.16.0"),
    ],
    targets: [
        .target(name: "YakumoCore"),
        .executableTarget(
            name: "Yakumo",
            dependencies: [
                "YakumoCore",
                .product(name: "KeyboardShortcuts", package: "KeyboardShortcuts"),
            ]
        ),
        .testTarget(name: "YakumoCoreTests", dependencies: ["YakumoCore"]),
    ]
)
