import Foundation
import os
@testable import YakumoCore

final class FakeHerdrAPI: HerdrAPI {
    enum Call: Equatable, Sendable {
        case list
        case focus(String)
        case create(WorktreeCreateParams)
        case remove(String)
        case listWorktrees(String)
        case focusAgent(String)
    }

    private struct State {
        var calls: [Call] = []
        var workspaces: [WorkspaceInfo] = []
        var listError: (any Error)?
        var focusError: (any Error)?
        var createResult: WorkspaceInfo?
        var createError: (any Error)?
        var removeError: (any Error)?
        var worktrees: [String: [WorktreeEntry]] = [:]
        var worktreesErrors: [String: any Error] = [:]
        var focusAgentError: (any Error)?
    }

    private let state = OSAllocatedUnfairLock(initialState: State())
    private let stream: AsyncStream<HerdrEvent>
    private let continuation: AsyncStream<HerdrEvent>.Continuation
    private let agentStream: AsyncStream<AgentStatusEvent>
    private let agentContinuation: AsyncStream<AgentStatusEvent>.Continuation

    init(workspaces: [WorkspaceInfo] = []) {
        (stream, continuation) = AsyncStream.makeStream(of: HerdrEvent.self)
        (agentStream, agentContinuation) = AsyncStream.makeStream(of: AgentStatusEvent.self)
        state.withLock { $0.workspaces = workspaces }
    }

    var calls: [Call] { state.withLock { $0.calls } }
    var listCount: Int { calls.filter { $0 == .list }.count }

    func send(_ event: HerdrEvent) { continuation.yield(event) }
    func finish() {
        continuation.finish()
        agentContinuation.finish()
    }
    func sendAgent(_ change: AgentStatusChange) { agentContinuation.yield(.changed(change)) }
    func sendBaseline(_ statuses: [String: AgentStatus]) { agentContinuation.yield(.baseline(statuses)) }
    func setFocusAgentError(_ error: (any Error)?) { state.withLock { $0.focusAgentError = error } }

    func setWorkspaces(_ workspaces: [WorkspaceInfo]) { state.withLock { $0.workspaces = workspaces } }
    func setListError(_ error: (any Error)?) { state.withLock { $0.listError = error } }
    func setFocusError(_ error: (any Error)?) { state.withLock { $0.focusError = error } }
    func setCreateResult(_ workspace: WorkspaceInfo) { state.withLock { $0.createResult = workspace } }
    func setCreateError(_ error: (any Error)?) { state.withLock { $0.createError = error } }
    func setRemoveError(_ error: (any Error)?) { state.withLock { $0.removeError = error } }
    func setWorktrees(_ entries: [WorktreeEntry], for repoRoot: String) {
        state.withLock { $0.worktrees[repoRoot] = entries }
    }
    func setWorktreesError(_ error: (any Error)?, for repoRoot: String) {
        state.withLock { $0.worktreesErrors[repoRoot] = error }
    }

    func listWorkspaces() async throws -> [WorkspaceInfo] {
        let (workspaces, error) = state.withLock { s in
            s.calls.append(.list)
            return (s.workspaces, s.listError)
        }
        if let error { throw error }
        return workspaces
    }

    func focusWorkspace(id: String) async throws {
        let error = state.withLock { s in
            s.calls.append(.focus(id))
            return s.focusError
        }
        if let error { throw error }
    }

    func createWorktree(_ params: WorktreeCreateParams) async throws -> WorkspaceInfo {
        let (result, error) = state.withLock { s in
            s.calls.append(.create(params))
            return (s.createResult, s.createError)
        }
        if let error { throw error }
        guard let result else { throw HerdrError(code: "fake", message: "no create result configured") }
        return result
    }

    func removeWorktree(workspaceID: String) async throws {
        let error = state.withLock { s in
            s.calls.append(.remove(workspaceID))
            return s.removeError
        }
        if let error { throw error }
    }

    func listWorktrees(repoRoot: String) async throws -> [WorktreeEntry] {
        let (entries, error) = state.withLock { s in
            s.calls.append(.listWorktrees(repoRoot))
            return (s.worktrees[repoRoot] ?? [], s.worktreesErrors[repoRoot])
        }
        if let error { throw error }
        return entries
    }

    func focusAgent(paneID: String) async throws {
        let error = state.withLock { s in
            s.calls.append(.focusAgent(paneID))
            return s.focusAgentError
        }
        if let error { throw error }
    }

    func events() -> AsyncStream<HerdrEvent> { stream }

    func agentStatusEvents() -> AsyncStream<AgentStatusEvent> { agentStream }
}
