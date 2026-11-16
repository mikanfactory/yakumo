import Foundation

public enum PathCompleter {
    private static let maxResults = 50

    public static func complete(
        _ input: String, home: String, fileManager: FileManager = .default
    ) -> [String] {
        guard !input.isEmpty else { return [] }
        let typed = input == "~" ? "~/" : input
        guard let slash = typed.lastIndex(of: "/") else { return [] }

        let typedParent = String(typed[...slash])
        let prefix = String(typed[typed.index(after: slash)...])
        let parent = expandTilde(typedParent, home: home)

        guard let names = try? fileManager.contentsOfDirectory(atPath: parent) else { return [] }
        let showHidden = prefix.hasPrefix(".")

        return names
            .filter { name in
                name.hasPrefix(prefix) && (showHidden || !name.hasPrefix("."))
                    && isDirectory(parent + name, fileManager)
            }
            .sorted()
            .prefix(maxResults)
            .map { typedParent + $0 + "/" }
    }

    private static func expandTilde(_ path: String, home: String) -> String {
        guard path.hasPrefix("~/") else { return path }
        let base = home.hasSuffix("/") ? String(home.dropLast()) : home
        return base + path.dropFirst(1)
    }

    // fileExists(atPath:) follows symlinks, so a link to a directory counts as one.
    private static func isDirectory(_ path: String, _ fileManager: FileManager) -> Bool {
        var isDir: ObjCBool = false
        return fileManager.fileExists(atPath: path, isDirectory: &isDir) && isDir.boolValue
    }
}
