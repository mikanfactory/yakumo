import Foundation
import os
import Testing
@testable import YakumoCore

private let prJSON = Data(
    #"{"number":7,"title":"t","state":"OPEN","url":"https://github.com/o/r/pull/7","mergeStateStatus":"CLEAN","reviewDecision":"","statusCheckRollup":[],"comments":[]}"#
        .utf8)

private func space(
    _ id: String, number: Int = 1, root: String? = "/repo", checkout: String? = nil, linked: Bool = true,
    focused: Bool = false, status: AgentStatus = .idle
) -> WorkspaceInfo {
    WorkspaceInfo(
        id: id, label: "label-\(id)", number: number, focused: focused, agentStatus: status,
        worktree: root.map {
            WorktreeInfo(checkoutPath: checkout ?? "/wt/\(id)", isLinkedWorktree: linked, repoName: "repo", repoRoot: $0)
        }
    )
}

@MainActor
private func waitUntil(timeout: Duration = .seconds(5), _ condition: () -> Bool) async -> Bool {
    let deadline = ContinuousClock.now + timeout
    while !condition() {
        guard ContinuousClock.now < deadline else { return false }
        try? await Task.sleep(for: .milliseconds(5))
    }
    return true
}

@MainActor
private func holds(for window: Duration, _ condition: () -> Bool) async -> Bool {
    let deadline = ContinuousClock.now + window
    while ContinuousClock.now < deadline {
        guard condition() else { return false }
        try? await Task.sleep(for: .milliseconds(5))
    }
    return condition()
}

private final class RunnerLog: Sendable {
    private let entries = OSAllocatedUnfairLock<[(args: [String], cwd: String)]>(initialState: [])
    var all: [(args: [String], cwd: String)] { entries.withLock { $0 } }
    func record(_ args: [String], _ cwd: String) { entries.withLock { $0.append((args, cwd)) } }
    func count(cwd: String) -> Int { all.filter { $0.cwd == cwd }.count }
}

private final class CommandLog: Sendable {
    private let entries = OSAllocatedUnfairLock<[(executable: String, args: [String])]>(initialState: [])
    var all: [(executable: String, args: [String])] { entries.withLock { $0 } }
    var gitCalls: [[String]] { all.filter { $0.executable == "/usr/bin/git" }.map(\.args) }
    func record(_ executable: String, _ args: [String]) { entries.withLock { $0.append((executable, args)) } }
}

private func createRunner(log: CommandLog, fetchStatus: Int32) -> CommandRunner {
    CommandRunner { executable, args, _ in
        log.record(executable, args)
        if executable == "/usr/bin/git" { return (fetchStatus, Data(), fetchStatus == 0 ? "" : "couldn't find remote ref") }
        return (0, Data(#"{"headRefName":"feat/pr-head"}"#.utf8), "")
    }
}

private struct ConfigBroken: Error {}

private enum HerdrWireKey: String, CodingKey { case workspaces }
private struct NotGit: Error {}

@MainActor
struct AppModelTests {
    private struct Harness {
        let model: AppModel
        let herdr: FakeHerdrAPI
        let runnerLog: RunnerLog
        let store: ConfigStore
        let task: Task<Void, Never>
    }

    private func makeStore(_ config: YakumoConfig? = nil) throws -> ConfigStore {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent("yakumo-appmodel-\(UUID().uuidString)/config.json")
        let store = ConfigStore(url: url)
        if let config { try store.save(config) }
        return store
    }

    private func makeHarness(
        workspaces: [WorkspaceInfo] = [],
        config: YakumoConfig? = nil,
        store: ConfigStore? = nil,
        runner: CommandRunner? = nil,
        gitToplevel: @escaping @Sendable (String) throws -> String = { $0 },
        statusInterval: Duration = .seconds(60),
        prInterval: Duration = .seconds(60),
        connect: Bool = true
    ) async throws -> Harness {
        let herdr = FakeHerdrAPI(workspaces: workspaces)
        let log = RunnerLog()
        let ghRunner = runner ?? CommandRunner { _, args, cwd in
            log.record(args, cwd)
            return (0, prJSON, "")
        }
        let configStore = try store ?? makeStore(config)
        let fetcher = PRFetcher(ghPath: "/gh", runner: ghRunner, now: { ContinuousClock.now })
        let model = AppModel(
            herdr: herdr, configStore: configStore, prFetcher: fetcher, runner: ghRunner,
            gitToplevel: gitToplevel, statusRefreshInterval: statusInterval, prRefreshInterval: prInterval)
        let task = Task { await model.start() }
        if connect {
            herdr.send(.connected)
            #expect(await waitUntil { model.connection == .connected && herdr.listCount >= 1 })
            #expect(await waitUntil { model.rows.count == ListBuilder.build(repositories: model.config.repositories, workspaces: workspaces).count })
        }
        return Harness(model: model, herdr: herdr, runnerLog: log, store: configStore, task: task)
    }

    private func moveCursor(_ model: AppModel, to id: String) {
        guard let index = model.rows.firstIndex(where: { $0.id == id }) else {
            Issue.record("row \(id) not found")
            return
        }
        let step = index > model.cursor ? 1 : -1
        while model.cursor != index {
            let before = model.cursor
            model.moveCursor(step)
            guard model.cursor != before else { break }
        }
        #expect(model.rows[model.cursor].id == id)
    }

    @Test func startsConnecting() async throws {
        let h = try await makeHarness(connect: false)
        defer { h.task.cancel() }
        #expect(h.model.connection == .connecting)
        #expect(h.model.message == nil)
    }

    @Test func disconnectedShowsUnavailableThenRecovers() async throws {
        let h = try await makeHarness(connect: false)
        defer { h.task.cancel() }
        h.herdr.send(.disconnected)
        #expect(await waitUntil { h.model.connection == .unavailable("herdr に接続できません") })
        #expect(h.herdr.listCount == 0)

        h.herdr.setWorkspaces([space("w1")])
        h.herdr.send(.connected)
        #expect(await waitUntil { h.model.connection == .connected })
        #expect(await waitUntil { h.model.rows.contains { $0.id == "w1" } })
        #expect(h.herdr.listCount == 1)
    }

    @Test func changedEventRebuildsRows() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        h.herdr.setWorkspaces([space("w1"), space("w2", number: 2)])
        h.herdr.send(.changed(kind: "workspace.created"))
        #expect(await waitUntil { h.model.rows.contains { $0.id == "w2" } })
        #expect(h.herdr.listCount == 2)
    }

    @Test func listFailureKeepsRowsAndShowsMessage() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        let before = h.model.rows
        h.herdr.setListError(HerdrError(code: "boom", message: "list failed"))
        h.herdr.send(.changed(kind: "workspace.updated"))
        #expect(await waitUntil { h.model.message == "list failed" })
        #expect(h.model.rows == before)
        #expect(h.model.connection == .connected)
    }

    @Test func nonHerdrErrorIsDescribedAndLoopKeepsRunning() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        let error = DecodingError.dataCorrupted(.init(codingPath: [], debugDescription: "bad json"))
        h.herdr.setListError(error)
        h.herdr.send(.changed(kind: "workspace.updated"))
        #expect(await waitUntil { h.model.message == "herdr の応答を読めません" })

        h.herdr.setListError(nil)
        h.herdr.setWorkspaces([space("w1"), space("w2", number: 2)])
        h.herdr.send(.changed(kind: "workspace.created"))
        #expect(await waitUntil { h.model.rows.contains { $0.id == "w2" } })
    }

    @Test func listFailureWhileDisconnectedKeepsEarlierMessage() async throws {
        let h = try await makeHarness(workspaces: [space("w1")], statusInterval: .milliseconds(10))
        defer { h.task.cancel() }
        h.herdr.send(.disconnected)
        #expect(await waitUntil { h.model.connection == .unavailable("herdr に接続できません") })
        h.model.message = "branch already exists"
        h.herdr.setListError(HerdrError(code: "connection_failed", message: "connect /x: Connection refused"))
        let before = h.herdr.listCount
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await waitUntil { h.herdr.listCount >= before + 3 })
        #expect(h.model.message == "branch already exists")
    }

    @Test func otherErrorsUseLocalizedDescription() {
        let error = CocoaError(.fileReadNoPermission)
        #expect(AppModel.describe(error) == error.localizedDescription)
        #expect(AppModel.describe(HerdrError(code: "x", message: "herdr said no")) == "herdr said no")
        let decoding = DecodingError.keyNotFound(
            HerdrWireKey.workspaces, .init(codingPath: [], debugDescription: "missing"))
        #expect(AppModel.describe(decoding) == "herdr の応答を読めません")
    }

    @Test func unreadableConfigIsNotBlamedOnHerdr() async throws {
        let store = try makeStore()
        try FileManager.default.createDirectory(
            at: store.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{broken".utf8).write(to: store.url)
        let h = try await makeHarness(store: store, connect: false)
        defer { h.task.cancel() }
        #expect(await waitUntil { h.model.message != nil })
        #expect(h.model.message?.contains("herdr") == false)
    }

    @Test func showingPanelMovesCursorToFocusedSpace() async throws {
        let h = try await makeHarness(workspaces: [
            space("w1", number: 1, focused: true), space("w2", number: 2), space("w3", number: 3),
        ])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w2")
        h.herdr.setWorkspaces([space("w1", number: 1), space("w2", number: 2), space("w3", number: 3, focused: true)])
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await waitUntil { h.model.rows[h.model.cursor].id == "w3" })

        moveCursor(h.model, to: "w1")
        await h.model.refresh()
        #expect(h.model.rows[h.model.cursor].id == "w1")
    }

    @Test func rebuildKeepsCursorOnSameRow() async throws {
        let h = try await makeHarness(workspaces: [space("w1", number: 1), space("w2", number: 2)])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w2")
        h.herdr.setWorkspaces([space("w0", number: 0), space("w1", number: 1), space("w2", number: 2)])
        h.herdr.send(.changed(kind: "workspace.created"))
        #expect(await waitUntil { h.model.rows.contains { $0.id == "w0" } })
        #expect(h.model.rows[h.model.cursor].id == "w2")
    }

    @Test func rebuildFallsBackToInitialCursorWhenRowDisappears() async throws {
        let h = try await makeHarness(workspaces: [space("w1", number: 1), space("w2", number: 2, focused: true)])
        defer { h.task.cancel() }
        #expect(h.model.rows[h.model.cursor].id == "w2")
        moveCursor(h.model, to: "w1")
        h.herdr.setWorkspaces([space("w2", number: 2, focused: true)])
        h.herdr.send(.changed(kind: "workspace.closed"))
        #expect(await waitUntil { !h.model.rows.contains { $0.id == "w1" } })
        #expect(h.model.cursor == Cursor.initial(in: h.model.rows))
        #expect(h.model.rows[h.model.cursor].id == "w2")
    }

    @Test func moveCursorSkipsHeaders() async throws {
        let h = try await makeHarness(workspaces: [space("w1", number: 1, root: "/a"), space("w2", number: 2, root: "/b")])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        h.model.moveCursor(1)
        #expect(h.model.rows[h.model.cursor].id == "w2")
    }

    @Test func removeOnNonLinkedSpaceDoesNothing() async throws {
        let h = try await makeHarness(workspaces: [
            space("main", number: 1, linked: false),
            space("plain", number: 2, root: nil),
        ])
        defer { h.task.cancel() }
        for id in ["main", "plain"] {
            h.model.message = nil
            moveCursor(h.model, to: id)
            #expect(h.model.requestRemove() == nil)
            #expect(h.model.message == "この Space は worktree ではないので削除できません")
        }
        #expect(!h.herdr.calls.contains { if case .remove = $0 { true } else { false } })
    }

    @Test func requestRemoveOnNonSpaceRowReturnsNilSilently() async throws {
        let h = try await makeHarness()
        defer { h.task.cancel() }
        moveCursor(h.model, to: "addRepository")
        #expect(h.model.requestRemove() == nil)
        #expect(h.model.message == nil)
    }

    @Test func removeErrorIsShownAndNotForced() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        let confirmation = try #require(h.model.requestRemove())
        #expect(confirmation == RemoveConfirmation(workspaceID: "w1", label: "label-w1"))
        h.herdr.setRemoveError(HerdrError(code: "dirty", message: "worktree has uncommitted changes"))
        await h.model.confirmRemove(confirmation)
        #expect(h.model.message == "worktree has uncommitted changes")
        #expect(h.herdr.calls.filter { $0 == .remove("w1") }.count == 1)
    }

    @Test func removeSuccessRelists() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        let confirmation = try #require(h.model.requestRemove())
        h.herdr.setWorkspaces([])
        await h.model.confirmRemove(confirmation)
        #expect(!h.model.rows.contains { $0.id == "w1" })
        #expect(h.model.message == nil)
    }

    @Test func createPassesBaseRefAndFocus() async throws {
        let h = try await makeHarness(config: .default)
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "")
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(cwd: "/repo", branch: nil, base: "origin/main", focus: true)])
        #expect(h.runnerLog.all.isEmpty)
    }

    @Test func createFromPRBranchStartsFromTheRemoteBranch() async throws {
        let log = CommandLog()
        let h = try await makeHarness(config: .default, runner: createRunner(log: log, fetchStatus: 0))
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "https://github.com/o/r/pull/9")
        #expect(log.gitCalls == [["-C", "/repo", "fetch", "origin", "feat/pr-head"]])
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(
            cwd: "/repo", branch: "feat/pr-head", base: "origin/feat/pr-head", focus: true)])
    }

    @Test func createFromRemoteBranchNameStartsFromTheRemoteBranch() async throws {
        let log = CommandLog()
        let h = try await makeHarness(config: .default, runner: createRunner(log: log, fetchStatus: 0))
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: " feat/x ")
        #expect(log.gitCalls == [["-C", "/repo", "fetch", "origin", "feat/x"]])
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(cwd: "/repo", branch: "feat/x", base: "origin/feat/x", focus: true)])
    }

    @Test func createNewBranchFallsBackToTheConfiguredBaseRef() async throws {
        let config = YakumoConfig(repositories: [], defaultBaseRef: "origin/develop", ghPath: "/gh")
        let log = CommandLog()
        let h = try await makeHarness(config: config, runner: createRunner(log: log, fetchStatus: 128))
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "feat/brand-new")
        #expect(log.gitCalls == [["-C", "/repo", "fetch", "origin", "feat/brand-new"]])
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(
            cwd: "/repo", branch: "feat/brand-new", base: "origin/develop", focus: true)])
        #expect(h.model.message == nil)
    }

    @Test func createNewBranchUsesDefaultBaseWhenFetchFails() async throws {
        let log = CommandLog()
        let h = try await makeHarness(config: .default, runner: createRunner(log: log, fetchStatus: 1))
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "feat/x")
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(cwd: "/repo", branch: "feat/x", base: "origin/main", focus: true)])
    }

    @Test func createNewBranchUsesDefaultBaseWhenGitCannotRun() async throws {
        let runner = CommandRunner { executable, _, _ in
            if executable == "/usr/bin/git" { throw NotGit() }
            return (0, prJSON, "")
        }
        let h = try await makeHarness(config: .default, runner: runner)
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "feat/x")
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(cwd: "/repo", branch: "feat/x", base: "origin/main", focus: true)])
    }

    @Test func createWithAutoNameDoesNotFetch() async throws {
        let log = CommandLog()
        let h = try await makeHarness(config: .default, runner: createRunner(log: log, fetchStatus: 0))
        defer { h.task.cancel() }
        h.herdr.setCreateResult(space("new"))
        await h.model.createWorktree(repoRoot: "/repo", input: "  ")
        #expect(log.all.isEmpty)
        let creates = h.herdr.calls.compactMap { if case .create(let p) = $0 { p } else { nil } }
        #expect(creates == [WorktreeCreateParams(cwd: "/repo", branch: nil, base: "origin/main", focus: true)])
    }

    @Test func createMovesCursorToNewSpace() async throws {
        let h = try await makeHarness(workspaces: [space("w1", number: 1)])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        let created = space("new", number: 2)
        h.herdr.setCreateResult(created)
        h.herdr.setWorkspaces([space("w1", number: 1), created])
        await h.model.createWorktree(repoRoot: "/repo", input: "feat/new")
        #expect(h.model.rows[h.model.cursor].id == "new")
    }

    @Test func createWithUnresolvablePRDoesNotCallHerdr() async throws {
        let runner = CommandRunner { _, _, _ in (1, Data(), "could not resolve to a PullRequest\nmore") }
        let h = try await makeHarness(runner: runner)
        defer { h.task.cancel() }
        await h.model.createWorktree(repoRoot: "/repo", input: "https://github.com/o/r/pull/9")
        #expect(h.model.message == "could not resolve to a PullRequest")
        #expect(!h.herdr.calls.contains { if case .create = $0 { true } else { false } })
    }

    @Test func createWithUnexpectedGhOutputShowsMessage() async throws {
        let runner = CommandRunner { _, _, _ in (0, Data("{}".utf8), "") }
        let h = try await makeHarness(runner: runner)
        defer { h.task.cancel() }
        await h.model.createWorktree(repoRoot: "/repo", input: "https://github.com/o/r/pull/9")
        #expect(h.model.message == "gh の出力を解釈できません")
        #expect(!h.herdr.calls.contains { if case .create = $0 { true } else { false } })
    }

    @Test func createErrorIsShown() async throws {
        let h = try await makeHarness()
        defer { h.task.cancel() }
        h.herdr.setCreateError(HerdrError(code: "exists", message: "branch already exists"))
        await h.model.createWorktree(repoRoot: "/repo", input: "feat/x")
        #expect(h.model.message == "branch already exists")
    }

    @Test func activateFocusesSpaceOnly() async throws {
        let h = try await makeHarness(config: YakumoConfig(
            repositories: [RepositoryDef(name: "repo", path: "/repo")], defaultBaseRef: "origin/main", ghPath: "/gh"))
        defer { h.task.cancel() }
        moveCursor(h.model, to: "add:/repo")
        #expect(await h.model.activate() == false)
        #expect(!h.herdr.calls.contains { if case .focus = $0 { true } else { false } })
    }

    @Test func activateFocusesSpace() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        #expect(await h.model.activate() == true)
        #expect(h.herdr.calls.contains(.focus("w1")))
    }

    @Test func activateFocusErrorKeepsPanelOpen() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        h.herdr.setFocusError(HerdrError(code: "not_found", message: "workspace not found"))
        #expect(await h.model.activate() == false)
        #expect(h.model.message == "workspace not found")
    }

    @Test func statusRefreshRunsOnlyWhilePanelVisible() async throws {
        let h = try await makeHarness(workspaces: [space("w1")], statusInterval: .milliseconds(10))
        defer { h.task.cancel() }
        let initial = h.herdr.listCount
        #expect(await holds(for: .milliseconds(60)) { h.herdr.listCount == initial })

        h.model.setPanelVisible(true)
        #expect(await waitUntil { h.herdr.listCount >= initial + 3 })
        h.herdr.setWorkspaces([space("w1", status: .working)])
        #expect(await waitUntil {
            h.model.rows.contains { if case .space(let w) = $0 { w.agentStatus == .working } else { false } }
        })

        h.model.setPanelVisible(false)
        let stopped = h.herdr.listCount
        #expect(await holds(for: .milliseconds(100)) { h.herdr.listCount <= stopped + 1 })
    }

    @Test func showingPanelTwiceDoesNotDoubleTheLoop() async throws {
        let h = try await makeHarness(workspaces: [space("w1")], statusInterval: .milliseconds(50))
        defer { h.task.cancel() }
        h.model.setPanelVisible(true)
        h.model.setPanelVisible(true)
        let start = h.herdr.listCount
        #expect(await holds(for: .milliseconds(120)) { h.herdr.listCount <= start + 4 })
        h.model.setPanelVisible(false)
    }

    @Test func prLookedUpWhenPanelOpens() async throws {
        let h = try await makeHarness(workspaces: [space("w1"), space("plain", number: 2, root: nil)])
        defer { h.task.cancel() }
        #expect(h.model.pr.isEmpty)
        #expect(h.runnerLog.all.isEmpty)
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await waitUntil { h.model.pr["/wt/w1"] != nil })
        guard case .found(let info) = h.model.pr["/wt/w1"] else {
            Issue.record("expected found PR")
            return
        }
        #expect(info.number == 7)
        #expect(h.model.pr.count == 1)
        #expect(h.runnerLog.all.map(\.cwd) == ["/wt/w1"])
    }

    @Test func reopeningPanelUsesFreshCache() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        h.model.setPanelVisible(true)
        #expect(await waitUntil { h.model.pr["/wt/w1"] != nil })
        h.model.setPanelVisible(false)
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await holds(for: .milliseconds(50)) { h.runnerLog.count(cwd: "/wt/w1") == 1 })
    }

    @Test func prRefreshesPeriodicallyWhileVisible() async throws {
        let h = try await makeHarness(workspaces: [space("w1")], prInterval: .milliseconds(20))
        defer { h.task.cancel() }
        h.model.setPanelVisible(true)
        #expect(await waitUntil { h.runnerLog.count(cwd: "/wt/w1") >= 3 })
        h.model.setPanelVisible(false)
        let stopped = h.runnerLog.count(cwd: "/wt/w1")
        #expect(await holds(for: .milliseconds(100)) { h.runnerLog.count(cwd: "/wt/w1") <= stopped + 1 })
    }

    @Test func refreshRelistsAndForcesPRLookup() async throws {
        let h = try await makeHarness(workspaces: [space("w1")])
        defer { h.task.cancel() }
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await waitUntil { h.runnerLog.count(cwd: "/wt/w1") == 1 })
        let lists = h.herdr.listCount
        await h.model.refresh()
        #expect(h.herdr.listCount > lists)
        #expect(await waitUntil { h.runnerLog.count(cwd: "/wt/w1") == 2 })
    }

    @Test func slowPRLookupDoesNotBlockRows() async throws {
        let gate = AsyncStream.makeStream(of: Void.self)
        let runner = CommandRunner { _, _, _ in
            for await _ in gate.stream { break }
            return (0, prJSON, "")
        }
        let h = try await makeHarness(workspaces: [space("w1")], runner: runner)
        defer {
            gate.continuation.finish()
            h.task.cancel()
        }
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        h.herdr.setWorkspaces([space("w1"), space("w2", number: 2)])
        await h.model.refresh()
        #expect(h.model.rows.contains { $0.id == "w2" })
        #expect(h.model.pr["/wt/w1"] == nil)
        gate.continuation.yield()
        #expect(await waitUntil { h.model.pr["/wt/w1"] != nil })
    }

    @Test func prURLForCursor() async throws {
        let noPR = CommandRunner { _, _, cwd in
            cwd == "/wt/w1" ? (0, prJSON, "") : (1, Data(), "no pull requests found for branch \"x\"")
        }
        let h = try await makeHarness(workspaces: [space("w1", number: 1), space("w2", number: 2)], runner: noPR)
        defer { h.task.cancel() }
        moveCursor(h.model, to: "w1")
        #expect(h.model.prURLForCursor() == nil)
        h.model.setPanelVisible(true)
        defer { h.model.setPanelVisible(false) }
        #expect(await waitUntil { h.model.pr.count == 2 })
        #expect(h.model.prURLForCursor() == URL(string: "https://github.com/o/r/pull/7"))
        moveCursor(h.model, to: "w2")
        #expect(h.model.prURLForCursor() == nil)
        moveCursor(h.model, to: "addRepository")
        #expect(h.model.prURLForCursor() == nil)
    }

    @Test func unreadableConfigShowsMessageAndUsesDefault() async throws {
        let store = try makeStore()
        try FileManager.default.createDirectory(
            at: store.url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{broken".utf8).write(to: store.url)
        let h = try await makeHarness(store: store, connect: false)
        defer { h.task.cancel() }
        #expect(await waitUntil { h.model.message?.hasPrefix("設定ファイルを読めません: ") == true })
        #expect(h.model.config == .default)
    }

    @Test func configRepositoriesShapeRows() async throws {
        let config = YakumoConfig(
            repositories: [RepositoryDef(name: "repo", path: "/repo")], defaultBaseRef: "origin/main", ghPath: "/gh")
        let h = try await makeHarness(workspaces: [space("w1")], config: config)
        defer { h.task.cancel() }
        #expect(h.model.rows.map(\.id) == ["repo:/repo", "w1", "add:/repo", "addRepository", "settings"])
    }

    @Test func addRepositoryStoresConfigAndRebuildsRows() async throws {
        let h = try await makeHarness(workspaces: [space("w1", root: "/code/app")], gitToplevel: { _ in "/code/app" })
        defer { h.task.cancel() }
        await h.model.addRepository(path: "/code/app/sub")
        #expect(h.model.config.repositories == [RepositoryDef(name: "app", path: "/code/app")])
        #expect(try h.store.load().repositories == [RepositoryDef(name: "app", path: "/code/app")])
        #expect(h.model.rows.map(\.id) == ["repo:/code/app", "w1", "add:/code/app", "addRepository", "settings"])
        #expect(h.model.message == nil)
    }

    @Test func addRepositoryRejectsNonGitPath() async throws {
        let h = try await makeHarness(gitToplevel: { _ in throw NotGit() })
        defer { h.task.cancel() }
        let rows = h.model.rows
        await h.model.addRepository(path: "/tmp/not-git")
        #expect(h.model.message == "リポジトリを追加できません: \(NotGit().localizedDescription)")
        #expect(h.model.config.repositories.isEmpty)
        #expect(h.model.rows == rows)
    }
}
