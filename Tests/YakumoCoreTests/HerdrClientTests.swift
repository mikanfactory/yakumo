import Foundation
import os
import Testing
@testable import YakumoCore

private let workspaceJSON = #"{"workspace_id":"w1","label":"main","number":1,"focused":true,"agent_status":"idle"}"#

private struct Request<Params: Decodable>: Decodable {
    let params: Params
}

private struct WorkspaceIDParams: Decodable, Equatable {
    let workspaceId: String
    let force: Bool?
}

private struct SubscribeParams: Decodable {
    struct Item: Decodable { let type: String }
    let subscriptions: [Item]
}

private func params<P: Decodable>(_ request: ReceivedRequest?, as type: P.Type) throws -> P {
    let line = try #require(request?.line)
    return try HerdrWire.makeDecoder().decode(Request<P>.self, from: line).params
}

private func waitUntil(timeout: Duration = .seconds(5), _ condition: () -> Bool) async throws -> Bool {
    let deadline = ContinuousClock.now + timeout
    while !condition() {
        guard ContinuousClock.now < deadline else { return false }
        try await Task.sleep(for: .milliseconds(5))
    }
    return true
}

private struct EventRecorder: Sendable {
    private let received = OSAllocatedUnfairLock<[HerdrEvent]>(initialState: [])
    private let task: Task<Void, Never>

    init(_ stream: AsyncStream<HerdrEvent>) {
        let received = received
        task = Task {
            for await event in stream {
                received.withLock { $0.append(event) }
            }
        }
    }

    var events: [HerdrEvent] { received.withLock { $0 } }

    func cancel() { task.cancel() }
}

@Suite struct HerdrClientTests {
    @Test func listWorkspacesSendsMethodAndDecodes() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.result(#"{"type":"workspace_list","workspaces":[\#(workspaceJSON),\#(workspaceJSON)]}"#), for: "workspace.list")

        let workspaces = try await HerdrClient(socketPath: server.path).listWorkspaces()

        #expect(server.requests.map(\.method) == ["workspace.list"])
        #expect(workspaces.count == 2)
        #expect(workspaces.first?.id == "w1")
    }

    @Test func errorResponseThrowsHerdrError() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.error(code: "not_found", message: "workspace w9 not found"), for: "workspace.focus")

        await #expect(throws: HerdrError(code: "not_found", message: "workspace w9 not found")) {
            try await HerdrClient(socketPath: server.path).focusWorkspace(id: "w9")
        }
    }

    @Test func missingSocketThrows() async {
        let client = HerdrClient(socketPath: "/tmp/yk-missing-\(UUID().uuidString.prefix(8)).sock")

        await #expect(throws: HerdrError.self) { try await client.listWorkspaces() }
    }

    @Test func peerClosingWithoutReplyThrows() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.hangUp, for: "workspace.list")

        let error = await #expect(throws: HerdrError.self) {
            try await HerdrClient(socketPath: server.path).listWorkspaces()
        }
        #expect(error?.code == "connection_closed")
    }

    @Test func silentPeerTimesOut() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.silence, for: "workspace.list")
        let client = HerdrClient(socketPath: server.path, requestTimeout: .milliseconds(100))

        let error = await #expect(throws: HerdrError.self) { try await client.listWorkspaces() }
        #expect(error?.code == "timeout")
    }

    @Test func slowCreateUsesTheLongTimeout() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let created = #"{"type":"worktree_created","workspace":\#(workspaceJSON),"worktree":{}}"#
        server.setReply(.delayedResult(created, after: .milliseconds(300)), for: "worktree.create")
        let client = HerdrClient(
            socketPath: server.path, requestTimeout: .milliseconds(100), worktreeTimeout: .seconds(5))

        let workspace = try await client.createWorktree(WorktreeCreateParams(cwd: "/repo", focus: true))

        #expect(workspace.id == "w1")
    }

    @Test func slowRemoveUsesTheLongTimeout() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.delayedResult(#"{"type":"ok"}"#, after: .milliseconds(300)), for: "worktree.remove")
        let client = HerdrClient(
            socketPath: server.path, requestTimeout: .milliseconds(100), worktreeTimeout: .seconds(5))

        try await client.removeWorktree(workspaceID: "w4")

        #expect(server.requests(method: "worktree.remove").count == 1)
    }

    @Test func slowListStillUsesTheShortTimeout() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let list = #"{"type":"workspace_list","workspaces":[]}"#
        server.setReply(.delayedResult(list, after: .milliseconds(300)), for: "workspace.list")
        let client = HerdrClient(
            socketPath: server.path, requestTimeout: .milliseconds(100), worktreeTimeout: .seconds(5))

        let error = await #expect(throws: HerdrError.self) { try await client.listWorkspaces() }
        #expect(error?.code == "timeout")
    }

    @Test func slowCreateTimesOutPastTheLongTimeout() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.silence, for: "worktree.create")
        let client = HerdrClient(
            socketPath: server.path, requestTimeout: .milliseconds(50), worktreeTimeout: .milliseconds(150))

        let error = await #expect(throws: HerdrError.self) {
            try await client.createWorktree(WorktreeCreateParams(cwd: "/repo", focus: true))
        }
        #expect(error?.code == "timeout")
    }

    @Test func focusWorkspaceSendsWorkspaceID() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }

        try await HerdrClient(socketPath: server.path).focusWorkspace(id: "w3")

        let request = server.requests(method: "workspace.focus").first
        #expect(try params(request, as: WorkspaceIDParams.self) == WorkspaceIDParams(workspaceId: "w3", force: nil))
    }

    @Test func removeWorktreeNeverForces() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }

        try await HerdrClient(socketPath: server.path).removeWorktree(workspaceID: "w4")

        let request = server.requests(method: "worktree.remove").first
        #expect(try params(request, as: WorkspaceIDParams.self) == WorkspaceIDParams(workspaceId: "w4", force: false))
    }

    @Test func createWorktreeSendsParamsAndReturnsWorkspace() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setReply(.result(#"{"type":"worktree_created","workspace":\#(workspaceJSON),"worktree":{}}"#), for: "worktree.create")

        let workspace = try await HerdrClient(socketPath: server.path)
            .createWorktree(WorktreeCreateParams(cwd: "/repo", base: "origin/main", focus: true))

        #expect(workspace.id == "w1")
        let line = try #require(server.requests(method: "worktree.create").first?.line)
        let text = String(decoding: line, as: UTF8.self)
        #expect(text.contains(#""cwd":"\/repo""#))
        #expect(text.contains(#""focus":true"#))
    }

    @Test func subscribesToEveryWorkspaceKind() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path).events())
        defer { recorder.cancel() }

        #expect(try await waitUntil { recorder.events == [.connected] })

        let subscribe = try params(server.requests(method: "events.subscribe").first, as: SubscribeParams.self)
        #expect(subscribe.subscriptions.map(\.type) == [
            "workspace.created", "workspace.updated", "workspace.closed", "workspace.focused",
            "workspace.renamed", "workspace.reordered", "worktree.created", "worktree.removed",
            "pane.agent_detected",
        ])
    }

    @Test func fakeRejectsAgentStatusSubscriptionWithoutPaneID() throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        var socket = try UnixSocket.connect(path: server.path, timeout: .seconds(5))
        defer { socket.close() }

        let params = ["subscriptions": [["type": "pane.agent_status_changed"]]]
        try socket.write(HerdrWire.encodeRequest(id: "s1", method: "events.subscribe", params: params))

        let reply = try HerdrWire.decodeLine(try #require(try socket.readLine()))
        guard case let .error(id, code, _) = reply else {
            Issue.record("expected error reply")
            return
        }
        #expect(id == "s1")
        #expect(code == "invalid_request")
    }

    @Test func clientSubscriptionIsAcceptedByServer() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path).events())
        defer { recorder.cancel() }

        #expect(try await waitUntil { recorder.events == [.connected] })
        #expect(server.subscriberCount == 1)
    }

    @Test func errorLineMidStreamReconnects() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path, reconnectInterval: .milliseconds(50)).events())
        defer { recorder.cancel() }
        #expect(try await waitUntil { recorder.events == [.connected] })
        let subscribeID = try #require(server.requests(method: "events.subscribe").first?.id)

        server.broadcast(#"{"id":"\#(subscribeID)","error":{"code":"events_lost","message":"subscriber too slow"}}"#)

        #expect(try await waitUntil { recorder.events == [.connected, .disconnected, .connected] })
        #expect(server.requests(method: "events.subscribe").count == 2)
    }

    @Test func eventLineBecomesChanged() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path).events())
        defer { recorder.cancel() }
        #expect(try await waitUntil { recorder.events == [.connected] })

        server.broadcast(#"{"event":"workspace.focused","data":{"workspace_id":"w1"}}"#)

        #expect(try await waitUntil { recorder.events == [.connected, .changed(kind: "workspace.focused")] })
    }

    @Test func eventsReconnectAfterServerRestart() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path, reconnectInterval: .milliseconds(50)).events())
        defer { recorder.cancel() }
        #expect(try await waitUntil { recorder.events == [.connected] })

        server.stop()
        #expect(try await waitUntil { recorder.events == [.connected, .disconnected] })
        try server.listen()

        #expect(try await waitUntil { recorder.events == [.connected, .disconnected, .connected] })
        #expect(server.requests(method: "events.subscribe").count == 2)
    }

    @Test func droppedSubscriptionReconnects() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path, reconnectInterval: .milliseconds(50)).events())
        defer { recorder.cancel() }
        #expect(try await waitUntil { recorder.events == [.connected] })

        server.dropSubscribers()

        #expect(try await waitUntil { recorder.events == [.connected, .disconnected, .connected] })
    }

    @Test func failedRetriesAnnounceDisconnectedOnce() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        server.setRejectsConnections(true)
        let recorder = EventRecorder(HerdrClient(socketPath: server.path, reconnectInterval: .milliseconds(10)).events())
        defer { recorder.cancel() }

        #expect(try await waitUntil { server.acceptedCount >= 3 })
        server.setRejectsConnections(false)

        #expect(try await waitUntil { recorder.events.last == .connected })
        #expect(recorder.events == [.disconnected, .connected])
    }

    @Test func missingSocketAnnouncesDisconnected() async throws {
        let client = HerdrClient(socketPath: "/tmp/yk-missing-\(UUID().uuidString.prefix(8)).sock", reconnectInterval: .milliseconds(10))
        let recorder = EventRecorder(client.events())
        defer { recorder.cancel() }

        #expect(try await waitUntil { recorder.events == [.disconnected] })
    }

    @Test func cancellingStreamClosesSubscription() async throws {
        let server = try FakeHerdrServer.started()
        defer { server.stop() }
        let recorder = EventRecorder(HerdrClient(socketPath: server.path).events())
        #expect(try await waitUntil { recorder.events == [.connected] })
        #expect(server.subscriberCount == 1)

        recorder.cancel()

        #expect(try await waitUntil { server.subscriberCount == 0 })
    }
}
