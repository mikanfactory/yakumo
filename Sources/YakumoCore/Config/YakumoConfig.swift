import Foundation

public struct YakumoConfig: Codable, Equatable, Sendable {
    public var defaultBaseRef: String
    public var ghPath: String

    public static let `default` = YakumoConfig(
        defaultBaseRef: "origin/main",
        ghPath: "~/.local/share/mise/shims/gh"
    )

    public init(defaultBaseRef: String, ghPath: String) {
        self.defaultBaseRef = defaultBaseRef
        self.ghPath = ghPath
    }

    enum CodingKeys: String, CodingKey {
        case defaultBaseRef = "default_base_ref"
        case ghPath = "gh_path"
    }

    // Hand-edited files may omit keys; synthesized decoding would reject them.
    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let d = YakumoConfig.default
        defaultBaseRef = try c.decodeIfPresent(String.self, forKey: .defaultBaseRef) ?? d.defaultBaseRef
        ghPath = try c.decodeIfPresent(String.self, forKey: .ghPath) ?? d.ghPath
    }
}
