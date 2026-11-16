import Foundation

public struct ConfigStore: Sendable {
    public let url: URL

    public init(url: URL) {
        self.url = url
    }

    public func load() throws -> YakumoConfig {
        guard FileManager.default.fileExists(atPath: url.path) else {
            return .default
        }
        return try JSONDecoder().decode(YakumoConfig.self, from: Data(contentsOf: url))
    }

    public func save(_ config: YakumoConfig) throws {
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
        try encoder.encode(config).write(to: url, options: .atomic)
    }

    public func addRepository(
        path: String,
        gitToplevel: (String) throws -> String
    ) throws -> YakumoConfig {
        let expanded = (path as NSString).expandingTildeInPath
        let toplevel = (try gitToplevel(expanded) as NSString).standardizingPath
        var config = try load()
        guard !config.repositories.contains(where: { $0.path == toplevel }) else {
            return config
        }
        let name = (toplevel as NSString).lastPathComponent
        config.repositories.append(RepositoryDef(name: name, path: toplevel))
        try save(config)
        return config
    }
}
