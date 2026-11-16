import Foundation

public actor PRFetcher {
    private static let jsonFields =
        "number,title,state,mergeStateStatus,reviewDecision,statusCheckRollup,comments,url"
    private static let noPRMarker = "no pull requests found"

    private struct Entry {
        let lookup: PRLookup
        let fetchedAt: ContinuousClock.Instant
    }

    private let ghPath: String
    private let runner: CommandRunner
    private let maxConcurrent: Int
    private let staleAfter: Duration
    private let now: @Sendable () -> ContinuousClock.Instant

    private var entries: [String: Entry] = [:]
    private var running = 0
    private var waiters: [CheckedContinuation<Void, Never>] = []

    public init(
        ghPath: String,
        runner: CommandRunner,
        maxConcurrent: Int = 4,
        staleAfter: Duration = .seconds(60),
        now: @escaping @Sendable () -> ContinuousClock.Instant
    ) {
        self.ghPath = (ghPath as NSString).expandingTildeInPath
        self.runner = runner
        self.maxConcurrent = maxConcurrent
        self.staleAfter = staleAfter
        self.now = now
    }

    public func cached(worktreePath: String) -> PRLookup? {
        entries[worktreePath]?.lookup
    }

    public func lookup(worktreePath: String, force: Bool) async -> PRLookup {
        if !force, let entry = entries[worktreePath], now() - entry.fetchedAt < staleAfter {
            return entry.lookup
        }
        await acquire()
        let result = await fetch(worktreePath: worktreePath)
        release()
        // A transient failure must not replace a PR the user already saw with an error.
        if case .failed = result {
            return entries[worktreePath]?.lookup ?? result
        }
        entries[worktreePath] = Entry(lookup: result, fetchedAt: now())
        return result
    }

    private func acquire() async {
        if running < maxConcurrent {
            running += 1
            return
        }
        await withCheckedContinuation { waiters.append($0) }
    }

    private func release() {
        if waiters.isEmpty {
            running -= 1
        } else {
            waiters.removeFirst().resume()
        }
    }

    private func fetch(worktreePath: String) async -> PRLookup {
        let output: (status: Int32, stdout: Data, stderr: String)
        do {
            output = try await runner.run(ghPath, ["pr", "view", "--json", Self.jsonFields], worktreePath)
        } catch {
            if error is CancellationError || Task.isCancelled {
                return .failed("キャンセルされました")
            }
            return .failed("gh が見つかりません: \(ghPath)")
        }
        guard output.status == 0 else {
            if output.stderr.contains(Self.noPRMarker) { return .none }
            return .failed("PR を取得できません: \(Self.firstLine(output.stderr))")
        }
        do {
            return .found(try JSONDecoder().decode(PRInfo.self, from: output.stdout))
        } catch {
            return .failed("PR を取得できません: 出力を解釈できません")
        }
    }

    static func firstLine(_ text: String) -> String {
        text.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
    }
}
