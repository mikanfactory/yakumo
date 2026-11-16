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
    func focusAgent(paneID: String) async throws
    func events() -> AsyncStream<HerdrEvent>
    func agentStatusEvents() -> AsyncStream<AgentStatusEvent>
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
    static let paneSetEvents: Set<String> = ["pane_created", "pane_closed"]

    private enum SubscriptionEnd: Equatable {
        case closed(subscribed: Bool)
        case resubscribe
    }

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

    public func focusAgent(paneID: String) async throws {
        _ = try await request("agent.focus", AgentTarget(target: paneID), timeout: requestTimeout)
    }

    public nonisolated func events() -> AsyncStream<HerdrEvent> {
        let items = Self.subscribedEventKinds.map { SubscribeParams.Item(type: $0) }
        return Self.makeStream { [socketPath, reconnectInterval] control, continuation in
            var announcedDisconnect = false
            await Self.retryUntilCancelled(interval: reconnectInterval, control: control) { attempt in
                Self.runSubscription(
                    path: socketPath, requestID: "subscribe-\(attempt)", items: items, control: control,
                    onSubscribed: { continuation.yield(.connected) },
                    onEvent: { kind, _ in
                        continuation.yield(.changed(kind: kind))
                        return false
                    }
                )
            } onClosed: { subscribed in
                if subscribed { announcedDisconnect = false }
                guard !announcedDisconnect else { return }
                continuation.yield(.disconnected)
                announcedDisconnect = true
            }
        }
    }

    public nonisolated func agentStatusEvents() -> AsyncStream<AgentStatusEvent> {
        Self.makeStream { [socketPath, reconnectInterval, requestTimeout] control, continuation in
            await Self.retryUntilCancelled(interval: reconnectInterval, control: control) { attempt in
                let panes: [PaneEntry]
                do {
                    let result = try Self.requestSync(
                        path: socketPath, id: "panes-\(attempt)", method: "pane.list", params: EmptyParams(),
                        timeout: requestTimeout)
                    panes = try HerdrWire.makeDecoder().decode(PaneList.self, from: result).panes
                } catch {
                    return .closed(subscribed: false)
                }
                let items = Self.paneSetKinds.map { SubscribeParams.Item(type: $0) }
                    + panes.map { SubscribeParams.Item(type: Self.agentStatusKind, paneID: $0.paneID) }
                let baseline = Dictionary(panes.map { ($0.paneID, $0.agentStatus) }, uniquingKeysWith: { $1 })
                return Self.runSubscription(
                    path: socketPath, requestID: "agents-\(attempt)", items: items, control: control,
                    onSubscribed: { continuation.yield(.baseline(baseline)) },
                    onEvent: { kind, data in
                        if Self.paneSetEvents.contains(kind) { return true }
                        if kind == Self.agentStatusKind,
                           let change = try? HerdrWire.makeDecoder().decode(AgentStatusChange.self, from: data) {
                            continuation.yield(.changed(change))
                        }
                        return false
                    }
                )
            } onClosed: { _ in }
        }
    }

    private func request(_ method: String, _ params: some Encodable & Sendable, timeout: Duration) async throws -> Data {
        nextRequestID += 1
        let id = String(nextRequestID)
        return try await offload { [socketPath] in
            try Self.requestSync(path: socketPath, id: id, method: method, params: params, timeout: timeout)
        }
    }

    private static func requestSync(
        path: String, id: String, method: String, params: some Encodable, timeout: Duration
    ) throws -> Data {
        let line = try HerdrWire.encodeRequest(id: id, method: method, params: params)
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

    private static func makeStream<Element: Sendable>(
        _ run: @escaping @Sendable (SubscriptionControl, AsyncStream<Element>.Continuation) async -> Void
    ) -> AsyncStream<Element> {
        let (stream, continuation) = AsyncStream.makeStream(of: Element.self)
        let control = SubscriptionControl()
        let task = Task.detached {
            await run(control, continuation)
            continuation.finish()
        }
        continuation.onTermination = { _ in
            control.cancel()
            task.cancel()
        }
        return stream
    }

    private static func retryUntilCancelled(
        interval: Duration, control: SubscriptionControl,
        attempt: @escaping @Sendable (Int) -> SubscriptionEnd,
        onClosed: (_ subscribed: Bool) -> Void
    ) async {
        var count = 0
        while !control.isCancelled {
            count += 1
            let current = count
            let end = await offload { attempt(current) }
            guard !control.isCancelled else { break }
            guard case let .closed(subscribed) = end else { continue }
            onClosed(subscribed)
            do {
                try await Task.sleep(for: interval)
            } catch {
                break
            }
        }
    }

    // onEvent returns true when the subscription has to be rebuilt.
    private static func runSubscription(
        path: String, requestID: String, items: [SubscribeParams.Item], control: SubscriptionControl,
        onSubscribed: () -> Void, onEvent: (_ kind: String, _ data: Data) -> Bool
    ) -> SubscriptionEnd {
        guard var socket = try? UnixSocket.connect(path: path, timeout: nil) else { return .closed(subscribed: false) }
        guard control.register(socket) else {
            socket.close()
            return .closed(subscribed: false)
        }
        defer {
            control.unregister()
            socket.close()
        }
        var subscribed = false
        do {
            try socket.write(HerdrWire.encodeRequest(
                id: requestID, method: "events.subscribe", params: SubscribeParams(subscriptions: items)))
            while let line = try socket.readLine() {
                switch try? HerdrWire.decodeLine(line) {
                case let .result(id, _) where id == requestID:
                    subscribed = true
                    onSubscribed()
                case let .error(_, code, message):
                    logger.error("herdr subscription error \(code, privacy: .public): \(message, privacy: .public)")
                    return .closed(subscribed: subscribed)
                case let .event(kind, data) where subscribed:
                    if onEvent(kind, data) { return .resubscribe }
                default:
                    continue
                }
            }
        } catch {}
        return .closed(subscribed: subscribed)
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

private struct PaneEntry: Decodable {
    let paneID: String
    let agentStatus: AgentStatus

    private enum CodingKeys: String, CodingKey {
        case paneID = "paneId"
        case agentStatus
    }
}

private struct PaneList: Decodable {
    let panes: [PaneEntry]
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
