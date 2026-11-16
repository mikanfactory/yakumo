import Foundation
import Observation

public enum ConnectionState: Equatable, Sendable {
    case connecting
    case connected
    case unavailable(String)
}

public struct RemoveConfirmation: Equatable, Sendable {
    public let workspaceID: String
    public let label: String

    public init(workspaceID: String, label: String) {
        self.workspaceID = workspaceID
        self.label = label
    }
}

@MainActor
@Observable
public final class AppModel {
    static let unavailableText = "herdr に接続できません"
    static let notWorktreeText = "この Space は worktree ではないので削除できません"
    static let gitPath = "/usr/bin/git"
    static let unreadableReplyText = "herdr の応答を読めません"

    public private(set) var rows: [ListRow] = []
    public private(set) var cursor = 0
    public private(set) var connection: ConnectionState = .connecting
    public var message: String?
    public private(set) var pr: [String: PRLookup] = [:]
    public private(set) var config: YakumoConfig = .default
    public private(set) var checkouts: [String: WorktreeEntry] = [:]

    @ObservationIgnored private let herdr: any HerdrAPI
    @ObservationIgnored private let configStore: ConfigStore
    @ObservationIgnored private let prFetcher: PRFetcher
    @ObservationIgnored private let runner: CommandRunner
    @ObservationIgnored private let statusRefreshInterval: Duration
    @ObservationIgnored private let prRefreshInterval: Duration

    @ObservationIgnored private var workspaces: [WorkspaceInfo] = []
    @ObservationIgnored private var listGeneration = 0
    @ObservationIgnored private var appliedGeneration = 0
    @ObservationIgnored private var panelTasks: [Task<Void, Never>] = []
    @ObservationIgnored private var prPending: Set<String> = []
    @ObservationIgnored private var cursorResetAfterGeneration: Int?

    public init(
        herdr: any HerdrAPI,
        configStore: ConfigStore,
        prFetcher: PRFetcher,
        runner: CommandRunner,
        statusRefreshInterval: Duration = .seconds(2),
        prRefreshInterval: Duration = .seconds(30)
    ) {
        self.herdr = herdr
        self.configStore = configStore
        self.prFetcher = prFetcher
        self.runner = runner
        self.statusRefreshInterval = statusRefreshInterval
        self.prRefreshInterval = prRefreshInterval
    }

    public func start() async {
        do {
            config = try configStore.load()
        } catch {
            message = "設定ファイルを読めません: \(error.localizedDescription)"
            config = .default
        }
        rebuildRows()
        for await event in herdr.events() {
            switch event {
            case .connected:
                connection = .connected
                await reloadWorkspaces()
            case .disconnected:
                connection = .unavailable(Self.unavailableText)
            case .changed:
                await reloadWorkspaces()
            }
        }
    }

    public func setPanelVisible(_ visible: Bool) {
        if !visible {
            panelTasks.forEach { $0.cancel() }
            panelTasks = []
            return
        }
        guard panelTasks.isEmpty else { return }
        cursorResetAfterGeneration = listGeneration
        lookupPRs(force: false)
        panelTasks = [
            repeating(every: statusRefreshInterval, immediately: true) { await $0.reloadWorkspaces() },
            repeating(every: prRefreshInterval, immediately: false) { $0.lookupPRs(force: true) },
        ]
    }

    public func moveCursor(_ delta: Int) {
        cursor = Cursor.move(cursor, by: delta, in: rows)
    }

    public func activate() async -> Bool {
        guard case .space(let workspace) = currentRow else { return false }
        do {
            try await herdr.focusWorkspace(id: workspace.id)
            return true
        } catch {
            message = Self.describe(error)
            return false
        }
    }

    public func createWorktree(repoRoot: String, input: String) async {
        let branch: String?
        do {
            branch = try await NewWorktreeInput.parse(input).resolveBranch(ghPath: config.ghPath, runner: runner)
        } catch {
            message = Self.describe(error)
            return
        }
        let base = if let branch { await baseRef(forBranch: branch, repoRoot: repoRoot) } else { config.defaultBaseRef }
        let params = WorktreeCreateParams(cwd: repoRoot, branch: branch, base: base, focus: true)
        do {
            let created = try await herdr.createWorktree(params)
            await reloadWorkspaces()
            if let index = rows.firstIndex(where: { $0.id == created.id }) {
                cursor = index
            }
        } catch {
            message = Self.describe(error)
        }
    }

    public func requestRemove() -> RemoveConfirmation? {
        guard case .space(let workspace) = currentRow else { return nil }
        guard workspace.worktree?.isLinkedWorktree == true else {
            message = Self.notWorktreeText
            return nil
        }
        return RemoveConfirmation(workspaceID: workspace.id, label: workspace.label)
    }

    public func confirmRemove(_ confirmation: RemoveConfirmation) async {
        do {
            try await herdr.removeWorktree(workspaceID: confirmation.workspaceID)
        } catch {
            message = Self.describe(error)
            return
        }
        await reloadWorkspaces()
    }

    public func refresh() async {
        await reloadWorkspaces()
        lookupPRs(force: true)
    }

    public func prURLForCursor() -> URL? {
        guard case .space(let workspace) = currentRow,
              let path = workspace.worktree?.checkoutPath,
              case .found(let info) = pr[path]
        else { return nil }
        return URL(string: info.url)
    }

    public func title(for workspace: WorkspaceInfo) -> String {
        SpaceTitle.title(for: workspace, checkouts: checkouts)
    }

    private var currentRow: ListRow? {
        rows.indices.contains(cursor) ? rows[cursor] : nil
    }

    private func reloadWorkspaces() async {
        listGeneration += 1
        let generation = listGeneration
        do {
            let listed = try await herdr.listWorkspaces()
            let fetchedCheckouts = await fetchCheckouts(for: listed)
            // An older list that finishes after a newer one would roll the rows back.
            guard generation > appliedGeneration else { return }
            let firstList = appliedGeneration == 0
            appliedGeneration = generation
            let resetCursor = cursorResetAfterGeneration.map { generation > $0 } ?? false
            if resetCursor { cursorResetAfterGeneration = nil }
            workspaces = listed
            checkouts = fetchedCheckouts
            rebuildRows(keepSelection: !firstList && !resetCursor)
            if !panelTasks.isEmpty {
                lookupPRs(force: false, onlyMissing: true)
            }
        } catch {
            // Not shown while disconnected: the connection state already says so, and a connect error
            // on every status tick would cover the message of the last real failure.
            if connection == .connected {
                message = Self.describe(error)
            }
        }
    }

    private func fetchCheckouts(for workspaces: [WorkspaceInfo]) async -> [String: WorktreeEntry] {
        let roots = Set(workspaces.compactMap(\.worktree?.repoRoot))
        return await withTaskGroup(of: [WorktreeEntry].self) { [herdr] group in
            for root in roots {
                // Not surfaced: a repo herdr cannot list only loses its branch name, the Space still shows.
                group.addTask { (try? await herdr.listWorktrees(repoRoot: root)) ?? [] }
            }
            var merged: [String: WorktreeEntry] = [:]
            for await entries in group {
                for entry in entries { merged[SpaceTitle.key(entry.path)] = entry }
            }
            return merged
        }
    }

    private func baseRef(forBranch branch: String, repoRoot: String) async -> String {
        let fetched = try? await runner.run(Self.gitPath, ["-C", repoRoot, "fetch", "origin", branch], repoRoot)
        return fetched?.status == 0 ? "origin/\(branch)" : config.defaultBaseRef
    }

    private func rebuildRows(keepSelection: Bool = true) {
        let selectedID = keepSelection ? currentRow?.id : nil
        rows = ListBuilder.build(workspaces: workspaces)
        if let selectedID, let index = rows.firstIndex(where: { $0.id == selectedID }) {
            cursor = index
        } else {
            cursor = Cursor.initial(in: rows)
        }
    }

    private func lookupPRs(force: Bool, onlyMissing: Bool = false) {
        let paths = Set(workspaces.compactMap(\.worktree?.checkoutPath))
        for path in paths {
            if onlyMissing, pr[path] != nil || prPending.contains(path) { continue }
            prPending.insert(path)
            Task { [prFetcher] in
                let result = await prFetcher.lookup(worktreePath: path, force: force)
                self.prPending.remove(path)
                self.pr[path] = result
            }
        }
    }

    private func repeating(
        every interval: Duration, immediately: Bool, _ work: @escaping @MainActor (AppModel) async -> Void
    ) -> Task<Void, Never> {
        Task { [weak self] in
            var first = immediately
            while !Task.isCancelled {
                if !first {
                    do { try await Task.sleep(for: interval) } catch { return }
                }
                first = false
                guard let self, !Task.isCancelled else { return }
                await work(self)
            }
        }
    }

    static func describe(_ error: any Error) -> String {
        switch error {
        case let error as HerdrError:
            error.message
        case NewWorktreeInputError.ghFailed(let line):
            line
        case NewWorktreeInputError.unexpectedOutput:
            "gh の出力を解釈できません"
        case is DecodingError:
            unreadableReplyText
        default:
            error.localizedDescription
        }
    }
}
