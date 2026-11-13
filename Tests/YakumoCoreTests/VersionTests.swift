import Testing
@testable import YakumoCore

@Test func versionIsSet() {
    #expect(!Yakumo.version.isEmpty)
}
