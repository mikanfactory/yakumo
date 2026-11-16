import Testing
@testable import YakumoCore

private func space(label: String = "repo", checkoutPath: String? = "/r", linked: Bool = false) -> WorkspaceInfo {
    WorkspaceInfo(
        id: "w1", label: label, number: 1, focused: false, agentStatus: .idle,
        worktree: checkoutPath.map {
            WorktreeInfo(checkoutPath: $0, isLinkedWorktree: linked, repoName: "repo", repoRoot: "/r")
        }
    )
}

private func checkouts(_ entries: WorktreeEntry...) -> [String: WorktreeEntry] {
    Dictionary(uniqueKeysWithValues: entries.map { (SpaceTitle.key($0.path), $0) })
}

@Suite struct SpaceTitleTests {
    @Test func primaryCheckoutShowsBranch() {
        let entries = checkouts(WorktreeEntry(path: "/r", branch: "main", isDetached: false, isLinkedWorktree: false))
        #expect(SpaceTitle.title(for: space(), checkouts: entries) == "main")
    }

    @Test func detachedShowsDetached() {
        let entries = checkouts(WorktreeEntry(path: "/r", branch: nil, isDetached: true, isLinkedWorktree: false))
        #expect(SpaceTitle.title(for: space(), checkouts: entries) == "(detached)")
    }

    @Test func linkedWorktreeKeepsLabel() {
        let entries = checkouts(WorktreeEntry(path: "/w/x", branch: "feat/x", isDetached: false, isLinkedWorktree: true))
        let workspace = space(label: "worktree-x", checkoutPath: "/w/x", linked: true)
        #expect(SpaceTitle.title(for: workspace, checkouts: entries) == "worktree-x")
    }

    @Test func missingEntryOrNonGitKeepsLabel() {
        #expect(SpaceTitle.title(for: space(label: "repo"), checkouts: [:]) == "repo")
        #expect(SpaceTitle.title(for: space(label: "scratch", checkoutPath: nil), checkouts: [:]) == "scratch")
    }

    @Test func emptyBranchKeepsLabel() {
        let entries = checkouts(WorktreeEntry(path: "/r", branch: "", isDetached: false, isLinkedWorktree: false))
        #expect(SpaceTitle.title(for: space(label: "repo"), checkouts: entries) == "repo")
    }

    @Test func pathSpellingIsNormalized() {
        let entries = checkouts(WorktreeEntry(path: "/r", branch: "main", isDetached: false, isLinkedWorktree: false))
        #expect(SpaceTitle.title(for: space(checkoutPath: "/r/"), checkouts: entries) == "main")
    }
}
