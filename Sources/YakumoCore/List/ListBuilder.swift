import Foundation

public enum ListBuilder {
    public static let otherHeaderName = "その他"

    public static func build(workspaces: [WorkspaceInfo]) -> [ListRow] {
        var rows: [ListRow] = []

        // Grouping by repoName would merge same-named repos checked out in different places.
        var roots: [String] = []
        var grouped: [String: [WorkspaceInfo]] = [:]
        for workspace in workspaces {
            guard let worktree = workspace.worktree else { continue }
            let root = standardize(worktree.repoRoot)
            if grouped[root] == nil { roots.append(root) }
            grouped[root, default: []].append(workspace)
        }
        for root in roots {
            let group = sorted(grouped[root] ?? [])
            guard let name = group.first?.worktree?.repoName else { continue }
            rows.append(.repoHeader(name: name, root: root))
            rows += group.map(ListRow.space)
            rows.append(.addWorktree(repoRoot: root))
        }

        let nonGit = sorted(workspaces.filter { $0.worktree == nil })
        if !nonGit.isEmpty {
            rows.append(.repoHeader(name: otherHeaderName, root: nil))
            rows += nonGit.map(ListRow.space)
        }

        rows.append(.settings)
        return rows
    }

    private static func sorted(_ workspaces: [WorkspaceInfo]) -> [WorkspaceInfo] {
        workspaces.sorted { $0.number < $1.number }
    }

    private static func standardize(_ path: String) -> String {
        (path as NSString).standardizingPath
    }
}
