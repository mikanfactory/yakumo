import Testing
@testable import YakumoCore

struct StatusSummaryTests {
    private func space(_ id: String, _ status: AgentStatus) -> ListRow {
        .space(WorkspaceInfo(id: id, label: id, number: 1, focused: false, agentStatus: status, worktree: nil))
    }

    @Test func countsEachActiveToneInFixedOrder() {
        let summary = StatusSummary(rows: [
            space("a", .done), space("b", .working), space("c", .blocked), space("d", .working), space("e", .idle),
        ])
        #expect(summary.entries == [
            .init(tone: .working, count: 2), .init(tone: .blocked, count: 1), .init(tone: .done, count: 1),
        ])
        #expect(summary.total == 5)
    }

    @Test func omitsTonesWithoutSpaces() {
        let summary = StatusSummary(rows: [space("a", .done), space("b", .idle)])
        #expect(summary.entries == [.init(tone: .done, count: 1)])
        #expect(summary.total == 2)
    }

    @Test func idleAndUnknownSpacesOnlyAddToTotal() {
        let summary = StatusSummary(rows: [space("a", .idle), space("b", .unknown)])
        #expect(summary.entries.isEmpty)
        #expect(summary.total == 2)
    }

    @Test func ignoresRowsThatAreNotSpaces() {
        let summary = StatusSummary(rows: [
            .repoHeader(name: "app", root: "/app"), space("a", .working), .addWorktree(repoRoot: "/app"), .settings,
        ])
        #expect(summary.entries == [.init(tone: .working, count: 1)])
        #expect(summary.total == 1)
    }

    @Test func emptyRowsGiveEmptySummary() {
        let summary = StatusSummary(rows: [])
        #expect(summary.entries.isEmpty)
        #expect(summary.total == 0)
    }
}
