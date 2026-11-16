import Testing
@testable import YakumoCore

private func notice(_ pane: String, _ status: AgentStatus = .done) -> AgentNotice {
    AgentNotice(paneID: pane, workspaceID: "w", name: "repo", detail: nil, isRepository: true, status: status)
}

@Suite struct NoticeStackTests {
    @Test func newestComesFirst() {
        var stack = NoticeStack(limit: 3)
        _ = stack.push(notice("a"))
        _ = stack.push(notice("b"))
        #expect(stack.notices.map(\.paneID) == ["b", "a"])
    }

    @Test func overflowDropsOldest() {
        var stack = NoticeStack(limit: 3)
        for pane in ["a", "b", "c"] { #expect(stack.push(notice(pane)).isEmpty) }

        #expect(stack.push(notice("d")) == ["a"])
        #expect(stack.notices.map(\.paneID) == ["d", "c", "b"])
    }

    @Test func samePaneReplacesAndMovesToTop() {
        var stack = NoticeStack(limit: 3)
        _ = stack.push(notice("a", .blocked))
        _ = stack.push(notice("b"))

        #expect(stack.push(notice("a", .done)).isEmpty)
        #expect(stack.notices == [notice("a", .done), notice("b")])
    }

    @Test func removeDropsOnlyThatPane() {
        var stack = NoticeStack(limit: 3)
        _ = stack.push(notice("a"))
        _ = stack.push(notice("b"))

        stack.remove(id: "a")
        stack.remove(id: "zz")

        #expect(stack.notices.map(\.paneID) == ["b"])
    }
}
