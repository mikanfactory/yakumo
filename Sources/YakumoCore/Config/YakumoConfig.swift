import Foundation

public struct RepositoryDef: Codable, Equatable, Sendable {
    public var name: String
    public var path: String

    public init(name: String, path: String) {
        self.name = name
        self.path = path
    }
}

public struct YakumoConfig: Codable, Equatable, Sendable {
    public var repositories: [RepositoryDef]
    public var defaultBaseRef: String
    public var ghPath: String

    public static let `default` = YakumoConfig(
        repositories: [],
        defaultBaseRef: "origin/main",
        ghPath: "~/.local/share/mise/shims/gh"
    )

    public init(repositories: [RepositoryDef], defaultBaseRef: String, ghPath: String) {
        self.repositories = repositories
        self.defaultBaseRef = defaultBaseRef
        self.ghPath = ghPath
    }

    enum CodingKeys: String, CodingKey {
        case repositories
        case defaultBaseRef = "default_base_ref"
        case ghPath = "gh_path"
    }

    // Hand-edited files may omit keys; synthesized decoding would reject them.
    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let d = YakumoConfig.default
        repositories = try c.decodeIfPresent([RepositoryDef].self, forKey: .repositories) ?? d.repositories
        defaultBaseRef = try c.decodeIfPresent(String.self, forKey: .defaultBaseRef) ?? d.defaultBaseRef
        ghPath = try c.decodeIfPresent(String.self, forKey: .ghPath) ?? d.ghPath
    }
}
