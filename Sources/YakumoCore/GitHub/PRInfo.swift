import Foundation

public struct CommandRunner: Sendable {
    public var run: @Sendable (_ executable: String, _ args: [String], _ cwd: String) async throws
        -> (status: Int32, stdout: Data, stderr: String)

    public init(
        run: @escaping @Sendable (_ executable: String, _ args: [String], _ cwd: String) async throws
            -> (status: Int32, stdout: Data, stderr: String)
    ) {
        self.run = run
    }
}

public enum CISummary: Equatable, Sendable {
    case passed
    case failed(count: Int)
    case running
    case none
}

public struct PRCheck: Decodable, Equatable, Sendable {
    public let name: String
    public let status: String
    public let result: String

    private enum CodingKeys: String, CodingKey {
        case name, context, state, status, conclusion
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        func text(_ key: CodingKeys) throws -> String {
            try c.decodeIfPresent(String.self, forKey: key) ?? ""
        }
        let name = try text(.name)
        self.name = name.isEmpty ? try text(.context) : name
        status = try text(.status)
        let conclusion = try text(.conclusion)
        result = conclusion.isEmpty ? try text(.state) : conclusion
    }

    private static let failureResults: Set<String> = [
        "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE",
    ]
    private static let successResults: Set<String> = ["SUCCESS", "SKIPPED", "NEUTRAL"]

    public var isFailure: Bool { Self.failureResults.contains(result) }
    public var isSuccess: Bool { Self.successResults.contains(result) }
}

public struct PRComment: Decodable, Equatable, Sendable {
    public let author: String
    public let body: String

    private enum CodingKeys: String, CodingKey { case author, body }
    private struct Author: Decodable { let login: String? }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        author = try c.decodeIfPresent(Author.self, forKey: .author)?.login ?? ""
        body = try c.decodeIfPresent(String.self, forKey: .body) ?? ""
    }
}

public struct PRInfo: Decodable, Equatable, Sendable {
    public let number: Int
    public let title: String
    public let state: String
    public let url: String
    public let mergeStateStatus: String
    public let reviewDecision: String
    public let checks: [PRCheck]
    public let comments: [PRComment]

    private enum CodingKeys: String, CodingKey {
        case number, title, state, url, mergeStateStatus, reviewDecision
        case checks = "statusCheckRollup"
        case comments
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        number = try c.decode(Int.self, forKey: .number)
        title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
        state = try c.decodeIfPresent(String.self, forKey: .state) ?? ""
        url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
        mergeStateStatus = try c.decodeIfPresent(String.self, forKey: .mergeStateStatus) ?? ""
        reviewDecision = try c.decodeIfPresent(String.self, forKey: .reviewDecision) ?? ""
        checks = try c.decodeIfPresent([PRCheck].self, forKey: .checks) ?? []
        comments = try c.decodeIfPresent([PRComment].self, forKey: .comments) ?? []
    }

    public var ciSummary: CISummary {
        guard !checks.isEmpty else { return .none }
        let failures = checks.filter(\.isFailure).count
        if failures > 0 { return .failed(count: failures) }
        // A check with no result yet is still running, whatever its status says.
        if checks.contains(where: { !$0.isSuccess }) { return .running }
        return .passed
    }

    public var mergeStateText: String {
        switch mergeStateStatus {
        case "CLEAN": "Ready to merge"
        case "BLOCKED": reviewDecision == "CHANGES_REQUESTED" ? "Changes requested" : "Blocked"
        case "BEHIND": "Behind base branch"
        case "UNSTABLE": "Checks failing"
        case "DIRTY": "Merge conflicts"
        default: mergeStateStatus
        }
    }
}

public enum PRLookup: Equatable, Sendable {
    case found(PRInfo)
    case none
    case failed(String)
}
