public enum AvatarPalette {
    public static let count = 8

    // Not String.hashValue: it is seeded per launch, so a repository would change colour on every restart.
    public static func index(for name: String) -> Int {
        var hash: UInt32 = 0x811c_9dc5
        for byte in name.utf8 {
            hash ^= UInt32(byte)
            hash = hash &* 0x0100_0193
        }
        return Int(hash % UInt32(count))
    }

    public static func initial(for name: String) -> String {
        name.first.map { String($0).uppercased() } ?? "?"
    }
}
