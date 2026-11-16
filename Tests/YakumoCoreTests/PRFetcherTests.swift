import Foundation
import Testing
@testable import YakumoCore

private final class Clock: @unchecked Sendable {
    private let lock = NSLock()
    private var current = ContinuousClock.now
    func now() -> ContinuousClock.Instant { lock.withLock { current } }
    func advance(_ d: Duration) { lock.withLock { current = current.advanced(by: d) } }
}

private let prJSON = Data(
    #"{"number":7,"title":"t","state":"OPEN","url":"https://x/7","mergeStateStatus":"CLEAN","reviewDecision":"","statusCheckRollup":[],"comments":[]}"#.utf8)

private let noPRStderr = #"no pull requests found for branch "feat/x""#

struct PRFetcherTests {
    private func makeFetcher(
        ghPath: String = "/gh", clock: Clock = Clock(), maxConcurrent: Int = 4,
        runner: CommandRunner
    ) -> PRFetcher {
        PRFetcher(ghPath: ghPath, runner: runner, maxConcurrent: maxConcurrent,
                  now: { clock.now() })
    }

    @Test func foundIsCachedAndPassesArguments() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, prJSON, "")
        }
        let fetcher = makeFetcher(runner: runner)
        let first = await fetcher.lookup(worktreePath: "/w/a", force: false)
        guard case .found(let pr) = first else { Issue.record("expected found"); return }
        #expect(pr.number == 7)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        let entries = await calls.entries
        #expect(entries.count == 1)
        #expect(entries[0].exe == "/gh")
        #expect(entries[0].cwd == "/w/a")
        #expect(entries[0].args == [
            "pr", "view", "--json",
            "number,title,state,mergeStateStatus,reviewDecision,statusCheckRollup,comments,url",
        ])
        #expect(await fetcher.cached(worktreePath: "/w/a") == first)
        #expect(await fetcher.cached(worktreePath: "/w/other") == nil)
    }

    @Test func noPullRequestIsCachedAsNone() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (1, Data(), noPRStderr)
        }
        let fetcher = makeFetcher(runner: runner)
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .none)
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .none)
        #expect(await calls.entries.count == 1)
        #expect(await fetcher.cached(worktreePath: "/w/a") == PRLookup.none)
    }

    @Test func ghMissingIsFailedNotThrown() async {
        struct Missing: Error {}
        let runner = CommandRunner { _, _, _ in throw Missing() }
        let fetcher = makeFetcher(ghPath: "/nope/gh", runner: runner)
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .failed("gh が見つかりません: /nope/gh"))
    }

    @Test func otherNonZeroExitIsFailedWithFirstStderrLineAndNotCached() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (4, Data(), "gh: To use GitHub CLI, run: gh auth login\nsecond")
        }
        let fetcher = makeFetcher(runner: runner)
        let expected = PRLookup.failed("PR を取得できません: gh: To use GitHub CLI, run: gh auth login")
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == expected)
        #expect(await fetcher.cached(worktreePath: "/w/a") == nil)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        #expect(await calls.entries.count == 2)
    }

    private actor Switch {
        var failing = false
        func fail() { failing = true }
    }

    private func flakyRunner(_ sw: Switch, error: (any Error)? = nil) -> CommandRunner {
        CommandRunner { _, _, _ in
            if await sw.failing {
                if let error { throw error }
                return (1, Data(), "network down")
            }
            return (0, prJSON, "")
        }
    }

    @Test func staleRefreshFailureKeepsCachedFound() async {
        let sw = Switch()
        let clock = Clock()
        let fetcher = makeFetcher(clock: clock, runner: flakyRunner(sw))
        let first = await fetcher.lookup(worktreePath: "/w/a", force: false)
        await sw.fail()
        clock.advance(.seconds(61))
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == first)
        #expect(await fetcher.cached(worktreePath: "/w/a") == first)
    }

    @Test func forcedRefreshFailureKeepsCachedFound() async {
        let sw = Switch()
        let fetcher = makeFetcher(runner: flakyRunner(sw))
        let first = await fetcher.lookup(worktreePath: "/w/a", force: false)
        await sw.fail()
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: true) == first)
        #expect(await fetcher.cached(worktreePath: "/w/a") == first)
    }

    @Test func refreshFailureKeepsCachedNone() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            if await calls.entries.count == 1 { return (1, Data(), noPRStderr) }
            return (1, Data(), "network down")
        }
        let fetcher = makeFetcher(runner: runner)
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .none)
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: true) == PRLookup.none)
        #expect(await fetcher.cached(worktreePath: "/w/a") == PRLookup.none)
    }

    @Test func failureWithoutCacheIsFailed() async {
        let sw = Switch()
        await sw.fail()
        let fetcher = makeFetcher(runner: flakyRunner(sw))
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .failed("PR を取得できません: network down"))
    }

    @Test func cancellationWithoutCacheIsCancelledNotMissing() async {
        let sw = Switch()
        await sw.fail()
        let fetcher = makeFetcher(runner: flakyRunner(sw, error: CancellationError()))
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: false) == .failed("キャンセルされました"))
    }

    @Test func cancellationWithCacheReturnsCache() async {
        let sw = Switch()
        let fetcher = makeFetcher(runner: flakyRunner(sw, error: CancellationError()))
        let first = await fetcher.lookup(worktreePath: "/w/a", force: false)
        await sw.fail()
        #expect(await fetcher.lookup(worktreePath: "/w/a", force: true) == first)
    }

    @Test func unparsableOutputIsFailed() async {
        let runner = CommandRunner { _, _, _ in (0, Data("garbage".utf8), "") }
        let fetcher = makeFetcher(runner: runner)
        guard case .failed(let message) = await fetcher.lookup(worktreePath: "/w/a", force: false) else {
            Issue.record("expected failed"); return
        }
        #expect(message.hasPrefix("PR を取得できません: "))
        #expect(!message.contains("\n"))
    }

    @Test func staleEntryRefetches() async {
        let calls = CallLog()
        let clock = Clock()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, prJSON, "")
        }
        let fetcher = makeFetcher(clock: clock, runner: runner)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        clock.advance(.seconds(59))
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        #expect(await calls.entries.count == 1)
        clock.advance(.seconds(1))
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        #expect(await calls.entries.count == 2)
    }

    @Test func forceRefetches() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, prJSON, "")
        }
        let fetcher = makeFetcher(runner: runner)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: true)
        #expect(await calls.entries.count == 2)
    }

    @Test func leadingTildeInGhPathIsExpandedBeforeRunning() async {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, prJSON, "")
        }
        let fetcher = makeFetcher(ghPath: "~/.local/share/mise/shims/gh", runner: runner)
        _ = await fetcher.lookup(worktreePath: "/w/a", force: false)
        #expect(await calls.entries.first?.exe == NSHomeDirectory() + "/.local/share/mise/shims/gh")
    }

    @Test func concurrencyIsCappedAt4() async {
        let gate = Gate()
        let runner = CommandRunner { _, _, _ in
            await gate.enter()
            return (0, prJSON, "")
        }
        let fetcher = makeFetcher(runner: runner)
        await withTaskGroup(of: Void.self) { group in
            for i in 0..<10 {
                group.addTask { _ = await fetcher.lookup(worktreePath: "/w/\(i)", force: false) }
            }
            let reached = await gate.waitUntilInFlight(4)
            #expect(reached)
            for _ in 0..<50 { await Task.yield() }
            #expect(await gate.inFlight == 4)
            await gate.releaseAll()
            await group.waitForAll()
        }
        #expect(await gate.maxInFlight == 4)
        #expect(await gate.total == 10)
    }
}

private actor Gate {
    private(set) var inFlight = 0
    private(set) var maxInFlight = 0
    private(set) var total = 0
    private var open = false
    private var held: [CheckedContinuation<Void, Never>] = []

    func enter() async {
        inFlight += 1
        total += 1
        maxInFlight = max(maxInFlight, inFlight)
        if !open {
            await withCheckedContinuation { held.append($0) }
        }
        inFlight -= 1
    }

    func waitUntilInFlight(_ n: Int, timeout: Duration = .seconds(5)) async -> Bool {
        let deadline = ContinuousClock.now.advanced(by: timeout)
        while inFlight < n {
            if ContinuousClock.now >= deadline { return false }
            try? await Task.sleep(for: .milliseconds(5))
        }
        return true
    }

    func releaseAll() {
        open = true
        let all = held
        held = []
        all.forEach { $0.resume() }
    }
}
