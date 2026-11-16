import Foundation

public enum ListRow: Equatable, Identifiable, Sendable {
    case repoHeader(name: String, root: String?)
    case space(WorkspaceInfo)
    case addWorktree(repoRoot: String)
    case addRepository
    case settings

    public var id: String {
        switch self {
        case .repoHeader(_, let root): "repo:\(root ?? "other")"
        case .space(let workspace): workspace.id
        case .addWorktree(let repoRoot): "add:\(repoRoot)"
        case .addRepository: "addRepository"
        case .settings: "settings"
        }
    }

    public var isSelectable: Bool {
        if case .repoHeader = self { return false }
        return true
    }
}
