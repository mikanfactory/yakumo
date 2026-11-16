import Foundation
import Testing
@testable import YakumoCore

struct ListBuilderTests {
    private func space(_ id: String, number: Int, root: String?, repoName: String = "app", focused: Bool = false) -> WorkspaceInfo {
        WorkspaceInfo(
            id: id, label: id, number: number, focused: focused, agentStatus: .idle,
            worktree: root.map { WorktreeInfo(checkoutPath: $0, isLinkedWorktree: false, repoName: repoName, repoRoot: $0) }
        )
    }

    @Test func groupsByRepoRootNotName() {
        let rows = ListBuilder.build(
            repositories: [],
            workspaces: [space("a", number: 1, root: "/x/app"), space("b", number: 2, root: "/y/app")]
        )
        #expect(rows.filter { if case .repoHeader = $0 { return true } else { return false } }.count == 2)
        #expect(rows[0] == .repoHeader(name: "app", root: "/x/app"))
        #expect(rows[1].id == "a")
        #expect(rows[2] == .repoHeader(name: "app", root: "/y/app"))
        #expect(rows[3].id == "b")
    }

    @Test func registeredReposKeepOrderAndSortSpacesByNumber() {
        let repos = [RepositoryDef(name: "B", path: "/b"), RepositoryDef(name: "A", path: "/a/")]
        let rows = ListBuilder.build(
            repositories: repos,
            workspaces: [space("a2", number: 3, root: "/a"), space("b1", number: 1, root: "/b"), space("a1", number: 2, root: "/a")]
        )
        #expect(rows.map(\.id) == [
            "repo:/b", "b1", "add:/b",
            "repo:/a", "a1", "a2", "add:/a",
            "addRepository", "settings",
        ])
    }

    @Test func registeredRepoWithoutSpacesStillShown() {
        let rows = ListBuilder.build(repositories: [RepositoryDef(name: "A", path: "/a")], workspaces: [])
        #expect(rows == [
            .repoHeader(name: "A", root: "/a"),
            .addWorktree(repoRoot: "/a"),
            .addRepository,
            .settings,
        ])
    }

    @Test func trailingSlashInRepoRootStillMatches() {
        let rows = ListBuilder.build(
            repositories: [RepositoryDef(name: "A", path: "/a")],
            workspaces: [space("s", number: 1, root: "/a/")]
        )
        #expect(rows.map(\.id) == ["repo:/a", "s", "add:/a", "addRepository", "settings"])
    }

    @Test func unregisteredAndNonGitGoLast() {
        let rows = ListBuilder.build(
            repositories: [RepositoryDef(name: "A", path: "/a")],
            workspaces: [
                space("none", number: 1, root: nil),
                space("other", number: 2, root: "/o", repoName: "other"),
                space("a", number: 3, root: "/a"),
            ]
        )
        #expect(rows == [
            .repoHeader(name: "A", root: "/a"), rows[1], .addWorktree(repoRoot: "/a"),
            .repoHeader(name: "other", root: "/o"), rows[4],
            .repoHeader(name: "その他", root: nil), rows[6],
            .addRepository, .settings,
        ])
        #expect(rows[1].id == "a")
        #expect(rows[4].id == "other")
        #expect(rows[6].id == "none")
    }

    @Test func rowIdsAreUniqueAndStableAcrossRebuilds() {
        let repos = [RepositoryDef(name: "A", path: "/a")]
        let spaces = [space("none", number: 1, root: nil), space("o", number: 2, root: "/o"), space("a", number: 3, root: "/a")]
        let rows = ListBuilder.build(repositories: repos, workspaces: spaces)
        #expect(Set(rows.map(\.id)).count == rows.count)
        #expect(rows.map(\.id) == ListBuilder.build(repositories: repos, workspaces: spaces).map(\.id))
        #expect(rows.contains { $0.id == "repo:other" })
    }
}
