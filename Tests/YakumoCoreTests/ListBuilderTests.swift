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
            workspaces: [space("a", number: 1, root: "/x/app"), space("b", number: 2, root: "/y/app")]
        )
        #expect(rows == [
            .repoHeader(name: "app", root: "/x/app"), rows[1], .addWorktree(repoRoot: "/x/app"),
            .repoHeader(name: "app", root: "/y/app"), rows[4], .addWorktree(repoRoot: "/y/app"),
            .settings,
        ])
        #expect(rows[1].id == "a")
        #expect(rows[4].id == "b")
    }

    @Test func reposKeepFirstAppearanceOrderAndSortSpacesByNumber() {
        let rows = ListBuilder.build(
            workspaces: [
                space("a2", number: 3, root: "/a", repoName: "A"),
                space("b1", number: 1, root: "/b", repoName: "B"),
                space("a1", number: 2, root: "/a", repoName: "A"),
            ]
        )
        #expect(rows.map(\.id) == [
            "repo:/a", "a1", "a2", "add:/a",
            "repo:/b", "b1", "add:/b",
            "settings",
        ])
    }

    @Test func noWorkspacesLeavesOnlySettings() {
        #expect(ListBuilder.build(workspaces: []) == [.settings])
    }

    @Test func trailingSlashInRepoRootMergesIntoOneGroup() {
        let rows = ListBuilder.build(
            workspaces: [space("s1", number: 1, root: "/a"), space("s2", number: 2, root: "/a/")]
        )
        #expect(rows.map(\.id) == ["repo:/a", "s1", "s2", "add:/a", "settings"])
    }

    @Test func nonGitGoesLastWithoutAddWorktree() {
        let rows = ListBuilder.build(
            workspaces: [
                space("none", number: 1, root: nil),
                space("a", number: 2, root: "/a", repoName: "A"),
            ]
        )
        #expect(rows == [
            .repoHeader(name: "A", root: "/a"), rows[1], .addWorktree(repoRoot: "/a"),
            .repoHeader(name: "その他", root: nil), rows[4],
            .settings,
        ])
        #expect(rows[1].id == "a")
        #expect(rows[4].id == "none")
    }

    @Test func rowIdsAreUniqueAndStableAcrossRebuilds() {
        let spaces = [space("none", number: 1, root: nil), space("o", number: 2, root: "/o"), space("a", number: 3, root: "/a")]
        let rows = ListBuilder.build(workspaces: spaces)
        #expect(Set(rows.map(\.id)).count == rows.count)
        #expect(rows.map(\.id) == ListBuilder.build(workspaces: spaces).map(\.id))
        #expect(rows.contains { $0.id == "repo:other" })
    }
}
