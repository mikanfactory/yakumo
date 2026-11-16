import Foundation

public enum AgentStatus: String, Decodable, Sendable {
    case idle, working, blocked, done, unknown

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = AgentStatus(rawValue: raw) ?? .unknown
    }
}

public struct WorktreeEntry: Decodable, Equatable, Sendable {
    public let path: String
    public let branch: String?
    public let isDetached: Bool
    public let isLinkedWorktree: Bool

    public init(path: String, branch: String?, isDetached: Bool, isLinkedWorktree: Bool) {
        self.path = path
        self.branch = branch
        self.isDetached = isDetached
        self.isLinkedWorktree = isLinkedWorktree
    }
}

public struct WorktreeInfo: Decodable, Equatable, Sendable {
    public let checkoutPath: String
    public let isLinkedWorktree: Bool
    public let repoName: String
    public let repoRoot: String

    public init(checkoutPath: String, isLinkedWorktree: Bool, repoName: String, repoRoot: String) {
        self.checkoutPath = checkoutPath
        self.isLinkedWorktree = isLinkedWorktree
        self.repoName = repoName
        self.repoRoot = repoRoot
    }
}

public struct WorkspaceInfo: Decodable, Equatable, Identifiable, Sendable {
    public let id: String
    public let label: String
    public let number: Int
    public let focused: Bool
    public let agentStatus: AgentStatus
    public let worktree: WorktreeInfo?

    public init(id: String, label: String, number: Int, focused: Bool, agentStatus: AgentStatus, worktree: WorktreeInfo?) {
        self.id = id
        self.label = label
        self.number = number
        self.focused = focused
        self.agentStatus = agentStatus
        self.worktree = worktree
    }

    // convertFromSnakeCase rewrites "workspace_id" to "workspaceId" before key matching.
    private enum CodingKeys: String, CodingKey {
        case id = "workspaceId"
        case label, number, focused, agentStatus, worktree
    }
}

public struct PaneInfo: Decodable, Equatable, Sendable {
    public let paneID: String
    public let workspaceID: String

    public init(paneID: String, workspaceID: String) {
        self.paneID = paneID
        self.workspaceID = workspaceID
    }

    private enum CodingKeys: String, CodingKey {
        case paneID = "paneId"
        case workspaceID = "workspaceId"
    }
}

public struct AgentStatusChange: Decodable, Equatable, Sendable {
    public let paneID: String
    public let workspaceID: String
    public let status: AgentStatus

    public init(paneID: String, workspaceID: String, status: AgentStatus) {
        self.paneID = paneID
        self.workspaceID = workspaceID
        self.status = status
    }

    private enum CodingKeys: String, CodingKey {
        case paneID = "paneId"
        case workspaceID = "workspaceId"
        case status = "agentStatus"
    }
}

public struct WorktreeCreateParams: Encodable, Equatable, Sendable {
    public var cwd: String?
    public var branch: String?
    public var base: String?
    public var focus: Bool

    public init(cwd: String? = nil, branch: String? = nil, base: String? = nil, focus: Bool) {
        self.cwd = cwd
        self.branch = branch
        self.base = base
        self.focus = focus
    }
}

public struct HerdrError: Error, Equatable, Sendable {
    public let code: String
    public let message: String

    public init(code: String, message: String) {
        self.code = code
        self.message = message
    }
}
