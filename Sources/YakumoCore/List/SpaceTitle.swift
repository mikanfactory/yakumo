import Foundation

public enum SpaceTitle {
    public static let detached = "(detached)"

    public static func key(_ path: String) -> String {
        (path as NSString).standardizingPath
    }

    public static func title(for workspace: WorkspaceInfo, checkouts: [String: WorktreeEntry]) -> String {
        guard let worktree = workspace.worktree, !worktree.isLinkedWorktree,
              let entry = checkouts[key(worktree.checkoutPath)]
        else { return workspace.label }
        if entry.isDetached { return detached }
        guard let branch = entry.branch, !branch.isEmpty else { return workspace.label }
        return branch
    }
}
