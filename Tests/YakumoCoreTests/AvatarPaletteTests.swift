import Testing
@testable import YakumoCore

@Suite struct AvatarPaletteTests {
    @Test func avatarIndexIsStable() {
        #expect(AvatarPalette.count == 8)
        #expect(AvatarPalette.index(for: "yakumo") == 7)
        #expect(AvatarPalette.index(for: "datahub") == 4)
        #expect(AvatarPalette.index(for: "databricks") == 3)
        #expect((0..<8).contains(AvatarPalette.index(for: "")))
    }

    @Test func avatarInitial() {
        #expect(AvatarPalette.initial(for: "yakumo") == "Y")
        #expect(AvatarPalette.initial(for: "") == "?")
        #expect(AvatarPalette.initial(for: "日本") == "日")
    }
}
