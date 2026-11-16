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
    func listWorktrees(repoRoot: String) async throws -> [WorktreeEntry]
    func listPanes() async throws -> [PaneInfo]
    func focusAgent(paneID: String) async throws
    func events() -> AsyncStream<HerdrEvent>
    func agentStatusChanges() -> AsyncStream<AgentStatusChange>
}

public actor HerdrClient: HerdrAPI {
    static let subscribedEventKinds = [
        "workspace.created", "workspace.updated", "workspace.closed", "workspace.focused",
        "workspace.renamed", "workspace.reordered", "worktree.created", "worktree.removed",
        "pane.agent_detected",
    ]
    static let agentStatusKind = "pane.agent_status_changed"
    // herdr only accepts status subscriptions per existing pane, so a new or closed pane means subscribing again.
    static let paneSetKinds = ["pane.created", "pane.closed"]
    static let paneSetEvents: Set<String> = ["pane_created", "pane_closed", "pane.created", "pane.closed"]

    private let socketPath: String
    private let reconnectInterval: Duration
    private let requestTimeout: Duration
    private let worktreeTimeout: Duration
    private var nextRequestID = 0

    public init(
        socketPath: String,
        reconnectInterval: Duration = .seconds(2),
        requestTimeout: Duration = .seconds(5),
        worktreeTimeout: Duration = .seconds(120)
    ) {
        self.socketPath = socketPath
        self.reconnectInterval = reconnectInterval
        self.requestTimeout = requestTimeout
        self.worktreeTimeout = worktreeTimeout
    }

    public func listWorkspaces() async throws -> [WorkspaceInfo] {
        let result = try await request("workspace.list", EmptyParams(), timeout: requestTimeout)
        return try HerdrWire.makeDecoder().decode(WorkspaceList.self, from: result).workspaces
    }

    public func focusWorkspace(id: String) async throws {
        _ = try await request("workspace.focus", WorkspaceParams(workspaceID: id, force: nil), timeout: requestTimeout)
    }

    public func createWorktree(_ params: WorktreeCreateParams) async throws -> WorkspaceInfo {
        let result = try await request("worktree.create", params, timeout: worktreeTimeout)
        return try HerdrWire.makeDecoder().decode(WorktreeCreated.self, from: result).workspace
    }

    public func removeWorktree(workspaceID: String) async throws {
        _ = try await request(
            "worktree.remove", WorkspaceParams(workspaceID: workspaceID, force: false), timeout: worktreeTimeout)
    }

    public func listWorktrees(repoRoot: String) async throws -> [WorktreeEntry] {
        let result = try await request("worktree.list", CwdParams(cwd: repoRoot), timeout: requestTimeout)
        return try HerdrWire.makeDecoder().decode(WorktreeList.self, from: result).worktrees
    }

    public func listPanes() async throws -> [PaneInfo] {
        let result = try await request("pane.list", EmptyParams(), timeout: requestTimeout)
        return try HerdrWire.makeDecoder().decode(PaneList.self, from: result).panes
    }

    public func focusAgent(paneID: String) async throws {
        _ = try await request("agent.focus", AgentTarget(target: paneID), timeout: requestTimeout)
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

    public nonisolated func agentStatusChanges() -> AsyncStream<AgentStatusChange> {
        let (stream, continuation) = AsyncStream.makeStream(of: AgentStatusChange.self)
        let control = SubscriptionControl()
        let task = Task.detached { [socketPath, reconnectInterval, requestTimeout] in
            await Self.watchAgentsUntilCancelled(
                path: socketPath, interval: reconnectInterval, timeout: requestTimeout, control: control,
                continuation: continuation
            )
        }
        continuation.onTermination = { _ in
            control.cancel()
            task.cancel()
        }
        return stream
    }

    private func request(_ method: String, _ params: some Encodable, timeout: Duration) async throws -> Data {
        nextRequestID += 1
        let id = String(nextRequestID)
        let line = try HerdrWire.encodeRequest(id: id, method: method, params: params)
        return try await offload { [socketPath] in
            try Self.requestSync(path: socketPath, id: id, method: method, line: line, timeout: timeout)
        }
    }

    private static func requestSync(path: String, id: String, method: String, line: Data, timeout: Duration) throws -> Data {
        var socket = try UnixSocket.connect(path: path, timeout: timeout)
        defer { socket.close() }
        try socket.write(line)
        switch try awaitReply(id: id, on: &socket) {
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
            let subscriptions = SubscribeParams(subscriptions: subscribedEventKinds.map { .init(type: $0) })
            try socket.write(HerdrWire.encodeRequest(id: requestID, method: "events.subscribe", params: subscriptions))
            while let line = try socket.readLine() {
                switch try? HerdrWire.decodeLine(line) {
                case let .result(id, _) where id == requestID:
                    connected = true
                    continuation.yield(.connected)
                case let .error(_, code, message):
                    logger.error("herdr subscription error \(code, privacy: .public): \(message, privacy: .public)")
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

extension HerdrClient {
    private enum AgentWatchOutcome {
        case paneSetChanged
        case ended
    }

    private static func watchAgentsUntilCancelled(
        path: String, interval: Duration, timeout: Duration, control: SubscriptionControl,
        continuation: AsyncStream<AgentStatusChange>.Continuation
    ) async {
        var attempt = 0
        while !control.isCancelled {
            attempt += 1
            let current = attempt
            let outcome = await offload {
                streamAgentStatus(path: path, attempt: current, timeout: timeout, control: control, continuation: continuation)
            }
            guard !control.isCancelled else { break }
            if outcome == .paneSetChanged { continue }
            do {
                try await Task.sleep(for: interval)
            } catch {
                break
            }
        }
        continuation.finish()
    }

    private static func streamAgentStatus(
        path: String, attempt: Int, timeout: Duration, control: SubscriptionControl,
        continuation: AsyncStream<AgentStatusChange>.Continuation
    ) -> AgentWatchOutcome {
        let panes: [PaneInfo]
        do {
            let id = "panes-\(attempt)"
            let line = try HerdrWire.encodeRequest(id: id, method: "pane.list", params: EmptyParams())
            let result = try requestSync(path: path, id: id, method: "pane.list", line: line, timeout: timeout)
            panes = try HerdrWire.makeDecoder().decode(PaneList.self, from: result).panes
        } catch {
            return .ended
        }
        guard var socket = try? UnixSocket.connect(path: path, timeout: nil) else { return .ended }
        guard control.register(socket) else {
            socket.close()
            return .ended
        }
        defer {
            control.unregister()
            socket.close()
        }
        let requestID = "agents-\(attempt)"
        let items = paneSetKinds.map { SubscribeParams.Item(type: $0) }
            + panes.map { SubscribeParams.Item(type: agentStatusKind, paneID: $0.paneID) }
        do {
            try socket.write(HerdrWire.encodeRequest(
                id: requestID, method: "events.subscribe", params: SubscribeParams(subscriptions: items)))
            var subscribed = false
            while let line = try socket.readLine() {
                switch try? HerdrWire.decodeLine(line) {
                case let .result(id, _) where id == requestID:
                    subscribed = true
                case let .error(_, code, message):
                    logger.error("herdr agent subscription error \(code, privacy: .public): \(message, privacy: .public)")
                    return .ended
                case let .event(kind, data) where subscribed && kind == agentStatusKind:
                    if let change = try? HerdrWire.makeDecoder().decode(AgentStatusChange.self, from: data) {
                        continuation.yield(change)
                    }
                case let .event(kind, _) where subscribed && paneSetEvents.contains(kind):
                    return .paneSetChanged
                default:
                    continue
                }
            }
        } catch {}
        return .ended
    }
}

private let logger = Logger(subsystem: "dev.mikanfactory.yakumo", category: "herdr")

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
        var paneID: String?

        enum CodingKeys: String, CodingKey {
            case type
            case paneID = "pane_id"
        }
    }

    let subscriptions: [Item]
}

private struct WorkspaceList: Decodable {
    let workspaces: [WorkspaceInfo]
}

private struct WorktreeCreated: Decodable {
    let workspace: WorkspaceInfo
}

private struct AgentTarget: Encodable {
    let target: String
}

private struct PaneList: Decodable {
    let panes: [PaneInfo]
}

private struct CwdParams: Encodable {
    let cwd: String
}

private struct WorktreeList: Decodable {
    let worktrees: [WorktreeEntry]
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
