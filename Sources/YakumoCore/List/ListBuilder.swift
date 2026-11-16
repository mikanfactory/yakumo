import Foundation

public enum ListBuilder {
    public static let otherHeaderName = "その他"

    public static func build(repositories: [RepositoryDef], workspaces: [WorkspaceInfo]) -> [ListRow] {
        var rows: [ListRow] = []
        let registeredRoots = Set(repositories.map { standardize($0.path) })

        for repository in repositories {
            let root = standardize(repository.path)
            rows.append(.repoHeader(name: repository.name, root: root))
            rows += sorted(workspaces.filter { $0.worktree.map { standardize($0.repoRoot) } == root }).map(ListRow.space)
            rows.append(.addWorktree(repoRoot: root))
        }

        // Grouping by repoName would merge same-named repos checked out in different places.
        var unregisteredRoots: [String] = []
        var unregistered: [String: [WorkspaceInfo]] = [:]
        for workspace in workspaces {
            guard let worktree = workspace.worktree else { continue }
            let root = standardize(worktree.repoRoot)
            if registeredRoots.contains(root) { continue }
            if unregistered[root] == nil { unregisteredRoots.append(root) }
            unregistered[root, default: []].append(workspace)
        }
        for root in unregisteredRoots {
            let group = sorted(unregistered[root] ?? [])
            guard let name = group.first?.worktree?.repoName else { continue }
            rows.append(.repoHeader(name: name, root: root))
            rows += group.map(ListRow.space)
        }

        let nonGit = sorted(workspaces.filter { $0.worktree == nil })
        if !nonGit.isEmpty {
            rows.append(.repoHeader(name: otherHeaderName, root: nil))
            rows += nonGit.map(ListRow.space)
        }

        rows.append(.addRepository)
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
