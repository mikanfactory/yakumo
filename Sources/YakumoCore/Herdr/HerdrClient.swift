import Foundation
import os

public enum HerdrEvent: Equatable, Sendable {
    case connected
    case disconnected
    case changed(kind: String)
}

public protocol HerdrAPI: Sendable {
    func listWorkspaces() async throws -> [WorkspaceInfo]
    func focusWorkspace(id: String) async throws
    func createWorktree(_ params: WorktreeCreateParams) async throws -> WorkspaceInfo
    func removeWorktree(workspaceID: String) async throws
    func events() -> AsyncStream<HerdrEvent>
}

public actor HerdrClient: HerdrAPI {
    static let subscribedEventKinds = [
        "workspace.created", "workspace.updated", "workspace.closed", "workspace.focused",
        "workspace.renamed", "workspace.reordered", "worktree.created", "worktree.removed",
        "pane.agent_status_changed",
    ]

    private let socketPath: String
    private let reconnectInterval: Duration
    private let requestTimeout: Duration
    private var nextRequestID = 0

    public init(socketPath: String, reconnectInterval: Duration = .seconds(2), requestTimeout: Duration = .seconds(5)) {
        self.socketPath = socketPath
        self.reconnectInterval = reconnectInterval
        self.requestTimeout = requestTimeout
    }

    public func listWorkspaces() async throws -> [WorkspaceInfo] {
        let result = try await request("workspace.list", EmptyParams())
        return try HerdrWire.makeDecoder().decode(WorkspaceList.self, from: result).workspaces
    }

    public func focusWorkspace(id: String) async throws {
        _ = try await request("workspace.focus", WorkspaceParams(workspaceID: id, force: nil))
    }

    public func createWorktree(_ params: WorktreeCreateParams) async throws -> WorkspaceInfo {
        let result = try await request("worktree.create", params)
        return try HerdrWire.makeDecoder().decode(WorktreeCreated.self, from: result).workspace
    }

    public func removeWorktree(workspaceID: String) async throws {
        _ = try await request("worktree.remove", WorkspaceParams(workspaceID: workspaceID, force: false))
    }

    public nonisolated func events() -> AsyncStream<HerdrEvent> {
        let (stream, continuation) = AsyncStream.makeStream(of: HerdrEvent.self)
        let control = SubscriptionControl()
        let task = Task.detached { [socketPath, reconnectInterval] in
            await Self.subscribeUntilCancelled(
                path: socketPath, interval: reconnectInterval, control: control, continuation: continuation
            )
        }
        continuation.onTermination = { _ in
            control.cancel()
            task.cancel()
        }
        return stream
    }

    private func request(_ method: String, _ params: some Encodable) async throws -> Data {
        nextRequestID += 1
        let id = String(nextRequestID)
        let line = try HerdrWire.encodeRequest(id: id, method: method, params: params)
        let message = try await offload { [socketPath, requestTimeout] in
            var socket = try UnixSocket.connect(path: socketPath, timeout: requestTimeout)
            defer { socket.close() }
            try socket.write(line)
            return try Self.awaitReply(id: id, on: &socket)
        }
        switch message {
        case let .result(_, json):
            return json
        case let .error(_, code, message):
            throw HerdrError(code: code, message: message)
        case .event:
            throw HerdrError(code: "malformed_message", message: "expected a reply to \(method)")
        }
    }

    private static func awaitReply(id: String, on socket: inout UnixSocket) throws -> HerdrMessage {
        while let line = try socket.readLine() {
            let message = try HerdrWire.decodeLine(line)
            switch message {
            case let .result(replyID, _) where replyID == id, let .error(replyID, _, _) where replyID == id:
                return message
            default:
                continue
            }
        }
        throw HerdrError(code: "connection_closed", message: "herdr closed the connection before replying")
    }

    private static func subscribeUntilCancelled(
        path: String, interval: Duration, control: SubscriptionControl, continuation: AsyncStream<HerdrEvent>.Continuation
    ) async {
        var announcedDisconnect = false
        var attempt = 0
        while !control.isCancelled {
            attempt += 1
            let id = "subscribe-\(attempt)"
            let connected = await offload {
                streamEvents(path: path, requestID: id, control: control, continuation: continuation)
            }
            if connected {
                announcedDisconnect = false
            }
            guard !control.isCancelled else { break }
            if !announcedDisconnect {
                continuation.yield(.disconnected)
                announcedDisconnect = true
            }
            do {
                try await Task.sleep(for: interval)
            } catch {
                break
            }
        }
        continuation.finish()
    }

    private static func streamEvents(
        path: String, requestID: String, control: SubscriptionControl, continuation: AsyncStream<HerdrEvent>.Continuation
    ) -> Bool {
        guard var socket = try? UnixSocket.connect(path: path, timeout: nil) else { return false }
        guard control.register(socket) else {
            socket.close()
            return false
        }
        defer {
            control.unregister()
            socket.close()
        }
        var connected = false
        do {
            let subscriptions = SubscribeParams(subscriptions: subscribedEventKinds.map(SubscribeParams.Item.init))
            try socket.write(HerdrWire.encodeRequest(id: requestID, method: "events.subscribe", params: subscriptions))
            while let line = try socket.readLine() {
                switch try? HerdrWire.decodeLine(line) {
                case let .result(id, _) where id == requestID:
                    connected = true
                    continuation.yield(.connected)
                case let .error(id, _, _) where id == requestID:
                    return connected
                case let .event(kind, _) where connected:
                    continuation.yield(.changed(kind: kind))
                default:
                    continue
                }
            }
        } catch {}
        return connected
    }
}

private struct EmptyParams: Encodable {}

private struct WorkspaceParams: Encodable {
    let workspaceID: String
    let force: Bool?

    enum CodingKeys: String, CodingKey {
        case workspaceID = "workspace_id"
        case force
    }
}

private struct SubscribeParams: Encodable {
    struct Item: Encodable {
        let type: String
    }

    let subscriptions: [Item]
}

private struct WorkspaceList: Decodable {
    let workspaces: [WorkspaceInfo]
}

private struct WorktreeCreated: Decodable {
    let workspace: WorkspaceInfo
}

private struct SubscriptionControl: Sendable {
    private struct State {
        var cancelled = false
        var socket: UnixSocket?
    }

    private let state = OSAllocatedUnfairLock(initialState: State())

    var isCancelled: Bool { state.withLock { $0.cancelled } }

    func cancel() {
        state.withLock { state in
            state.cancelled = true
            state.socket?.shutdown()
        }
    }

    func register(_ socket: UnixSocket) -> Bool {
        state.withLock { state in
            guard !state.cancelled else { return false }
            state.socket = socket
            return true
        }
    }

    // Does not close the socket itself: the reader closes it after this, so cancel() never shuts down a reused fd.
    func unregister() {
        state.withLock { $0.socket = nil }
    }
}

private func offload<T: Sendable>(_ work: @escaping @Sendable () throws -> T) async throws -> T {
    try await withCheckedThrowingContinuation { continuation in
        DispatchQueue.global().async {
            continuation.resume(with: Result { try work() })
        }
    }
}

private func offload<T: Sendable>(_ work: @escaping @Sendable () -> T) async -> T {
    await withCheckedContinuation { continuation in
        DispatchQueue.global().async {
            continuation.resume(returning: work())
        }
    }
}
