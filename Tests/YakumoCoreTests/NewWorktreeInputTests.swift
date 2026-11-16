import Foundation
import Testing
@testable import YakumoCore

struct NewWorktreeInputTests {
    @Test func parseInput() {
        #expect(NewWorktreeInput.parse("") == .auto)
        #expect(NewWorktreeInput.parse("   \n") == .auto)
        #expect(NewWorktreeInput.parse("feat/x") == .branch("feat/x"))
        #expect(NewWorktreeInput.parse("  feat/x  ") == .branch("feat/x"))
        #expect(
            NewWorktreeInput.parse(" https://github.com/o/r/pull/12 ")
                == .pullRequest(URL(string: "https://github.com/o/r/pull/12")!))
    }

    @Test func autoResolvesToNilWithoutCallingRunner() async throws {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, Data(), "")
        }
        let branch = try await NewWorktreeInput.auto.resolveBranch(ghPath: "gh", runner: runner)
        #expect(branch == nil)
        #expect(await calls.entries.isEmpty)
    }

    @Test func branchResolvesToItself() async throws {
        let runner = CommandRunner { _, _, _ in Issue.record("runner must not be called"); return (0, Data(), "") }
        let branch = try await NewWorktreeInput.branch("feat/x").resolveBranch(ghPath: "gh", runner: runner)
        #expect(branch == "feat/x")
    }

    @Test func pullRequestResolvesHeadRefNameWithExpandedGhPath() async throws {
        let calls = CallLog()
        let runner = CommandRunner { exe, args, cwd in
            await calls.record(exe, args, cwd)
            return (0, Data(#"{"headRefName":"feat/y"}"#.utf8), "")
        }
        let url = URL(string: "https://github.com/o/r/pull/12")!
        let branch = try await NewWorktreeInput.pullRequest(url)
            .resolveBranch(ghPath: "~/bin/gh", runner: runner)
        #expect(branch == "feat/y")
        let entry = try #require(await calls.entries.first)
        #expect(entry.exe == NSHomeDirectory() + "/bin/gh")
        #expect(entry.args == ["pr", "view", "https://github.com/o/r/pull/12", "--json", "headRefName"])
    }

    @Test func pullRequestFailureThrowsWithStderr() async {
        let runner = CommandRunner { _, _, _ in (1, Data(), "boom\nmore") }
        let url = URL(string: "https://github.com/o/r/pull/12")!
        await #expect(throws: NewWorktreeInputError.ghFailed("boom")) {
            _ = try await NewWorktreeInput.pullRequest(url).resolveBranch(ghPath: "gh", runner: runner)
        }
    }

    @Test func pullRequestWithUnparsableOutputThrows() async {
        let runner = CommandRunner { _, _, _ in (0, Data("nope".utf8), "") }
        let url = URL(string: "https://github.com/o/r/pull/12")!
        await #expect(throws: NewWorktreeInputError.unexpectedOutput) {
            _ = try await NewWorktreeInput.pullRequest(url).resolveBranch(ghPath: "gh", runner: runner)
        }
    }
}

actor CallLog {
    struct Entry: Equatable { let exe: String; let args: [String]; let cwd: String }
    private(set) var entries: [Entry] = []
    func record(_ exe: String, _ args: [String], _ cwd: String) {
        entries.append(Entry(exe: exe, args: args, cwd: cwd))
    }
}
