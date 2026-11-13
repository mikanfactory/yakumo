import Foundation
import Testing
@testable import YakumoCore

struct CursorTests {
    private func space(_ id: String, focused: Bool = false) -> ListRow {
        .space(WorkspaceInfo(id: id, label: id, number: 1, focused: focused, agentStatus: .idle, worktree: nil))
    }

    private var rows: [ListRow] {
        [.repoHeader(name: "A", root: "/a"), space("a1"), space("a2"), .repoHeader(name: "B", root: "/b"), space("b1"), .addRepository]
    }

    @Test func cursorSkipsHeadersAndClamps() {
        #expect(Cursor.move(1, by: 1, in: rows) == 2)
        #expect(Cursor.move(2, by: 1, in: rows) == 4)
        #expect(Cursor.move(4, by: -1, in: rows) == 2)
        #expect(Cursor.move(1, by: -1, in: rows) == 1)
        #expect(Cursor.move(5, by: 1, in: rows) == 5)
        #expect(Cursor.move(0, by: 1, in: rows) == 1)
    }

    @Test func emptyAndHeaderOnlyListsKeepIndex() {
        #expect(Cursor.move(0, by: 1, in: []) == 0)
        let headers: [ListRow] = [.repoHeader(name: "A", root: "/a")]
        #expect(Cursor.move(0, by: 1, in: headers) == 0)
        #expect(Cursor.initial(in: []) == 0)
        #expect(Cursor.initial(in: headers) == 0)
    }

    @Test func initialCursorIsFocusedSpace() {
        let withFocus: [ListRow] = [.repoHeader(name: "A", root: "/a"), space("a1"), space("a2", focused: true)]
        #expect(Cursor.initial(in: withFocus) == 2)
    }

    @Test func initialCursorFallsBackToFirstSelectable() {
        #expect(Cursor.initial(in: rows) == 1)
    }
}
