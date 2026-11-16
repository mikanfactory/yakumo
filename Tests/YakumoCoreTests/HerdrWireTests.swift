import Foundation
import Testing
@testable import YakumoCore

private struct WorkspaceListResult: Decodable {
    let workspaces: [WorkspaceInfo]
}

private func fixtureLine() throws -> Data {
    let url = try #require(Bundle.module.url(forResource: "workspace_list", withExtension: "json", subdirectory: "Fixtures"))
    return try Data(contentsOf: url)
}

@Test func decodesWorkspaceList() throws {
    let message = try HerdrWire.decodeLine(fixtureLine())
    guard case let .result(_, json) = message else {
        Issue.record("expected result")
        return
    }
    let list = try HerdrWire.makeDecoder().decode(WorkspaceListResult.self, from: json)
    #expect(list.workspaces.count == 8)
    let target = try #require(list.workspaces.first { $0.label == "worktree-clear-stone-a132" })
    #expect(target.agentStatus == .working)
    #expect(target.worktree?.isLinkedWorktree == true)
    #expect(target.id.isEmpty == false)
}

@Test func unknownAgentStatusFallsBack() throws {
    let json = Data(#"{"workspace_id":"w1","label":"a","number":1,"focused":false,"agent_status":"sleeping"}"#.utf8)
    let info = try HerdrWire.makeDecoder().decode(WorkspaceInfo.self, from: json)
    #expect(info.agentStatus == .unknown)
    #expect(info.worktree == nil)
    #expect(info.id == "w1")
}

@Test func encodesRequestAsSingleLine() throws {
    let data = try HerdrWire.encodeRequest(id: "1", method: "workspace.focus", params: ["workspace_id": "wP"])
    let text = try #require(String(data: data, encoding: .utf8))
    #expect(text.hasSuffix("\n"))
    #expect(text.filter { $0 == "\n" }.count == 1)
    #expect(text.contains(#""method":"workspace.focus""#))
    #expect(text.contains(#""workspace_id":"wP""#))
}

@Test func encodesWorktreeCreateParams() throws {
    let params = WorktreeCreateParams(cwd: "/r", branch: nil, base: "origin/main", focus: true)
    let data = try HerdrWire.encodeRequest(id: "1", method: "worktree.create", params: params)
    let text = try #require(String(data: data, encoding: .utf8))
    #expect(text.contains(#""base":"origin\/main""#) || text.contains(#""base":"origin/main""#))
    #expect(!text.contains("branch"))
    #expect(text.contains(#""focus":true"#))
}

@Test func decodesErrorAndEvent() throws {
    let error = try HerdrWire.decodeLine(Data(#"{"id":"2","error":{"code":"not_found","message":"x"}}"#.utf8))
    #expect(matches(error, .error(id: "2", code: "not_found", message: "x")))

    let event = try HerdrWire.decodeLine(Data(#"{"event":"workspace.focused","data":{}}"#.utf8))
    guard case let .event(kind, data) = event else {
        Issue.record("expected event")
        return
    }
    #expect(kind == "workspace.focused")
    #expect(String(data: data, encoding: .utf8) == "{}")
}

@Test func decodeLineRejectsGarbage() {
    #expect(throws: (any Error).self) { try HerdrWire.decodeLine(Data("nope".utf8)) }
    #expect(throws: HerdrError.self) { try HerdrWire.decodeLine(Data(#"{"foo":1}"#.utf8)) }
}

private func matches(_ message: HerdrMessage, _ expected: HerdrMessage) -> Bool {
    switch (message, expected) {
    case let (.error(a, b, c), .error(x, y, z)): a == x && b == y && c == z
    default: false
    }
}
