import Foundation
import Testing
@testable import YakumoCore

struct PRInfoTests {
    private func fixture(_ name: String) throws -> Data {
        let url = try #require(Bundle.module.url(forResource: name, withExtension: "json", subdirectory: "Fixtures"))
        return try Data(contentsOf: url)
    }

    private func decode(_ json: String) throws -> PRInfo {
        try JSONDecoder().decode(PRInfo.self, from: Data(json.utf8))
    }

    @Test func decodesFixtureAndSummarizesCI() throws {
        let pr = try JSONDecoder().decode(PRInfo.self, from: fixture("gh_pr_view"))
        #expect(pr.number == 14927)
        #expect(pr.state == "OPEN")
        #expect(pr.checks.count == 25)
        #expect(pr.comments.isEmpty)
        #expect(pr.ciSummary == .failed(count: 1))
        #expect(pr.mergeStateText == "Blocked")
    }

    @Test func decodesCommentsFixture() throws {
        let pr = try JSONDecoder().decode(PRInfo.self, from: fixture("gh_pr_view_comments"))
        #expect(pr.number == 15046)
        #expect(pr.comments.count == 1)
        #expect(pr.comments[0].author == "github-actions")
        #expect(!pr.comments[0].body.isEmpty)
        #expect(pr.ciSummary == .passed)
    }

    @Test(arguments: [
        ("CLEAN", "", "Ready to merge"),
        ("BLOCKED", "CHANGES_REQUESTED", "Changes requested"),
        ("BLOCKED", "REVIEW_REQUIRED", "Blocked"),
        ("BLOCKED", "", "Blocked"),
        ("BEHIND", "", "Behind base branch"),
        ("UNSTABLE", "", "Checks failing"),
        ("DIRTY", "", "Merge conflicts"),
        ("HAS_HOOKS", "", "HAS_HOOKS"),
        ("", "", ""),
    ])
    func mergeStateTextMatchesGoTable(merge: String, review: String, expected: String) throws {
        let pr = try decode(
            #"{"number":1,"title":"t","state":"OPEN","url":"u","mergeStateStatus":"\#(merge)","reviewDecision":"\#(review)"}"#)
        #expect(pr.mergeStateText == expected)
    }

    @Test func missingOptionalKeysDecodeAsEmpty() throws {
        let pr = try decode(#"{"number":2,"title":"t","state":"OPEN","url":"u"}"#)
        #expect(pr.checks.isEmpty)
        #expect(pr.comments.isEmpty)
        #expect(pr.ciSummary == .none)
        #expect(pr.mergeStateText == "")
    }

    private func summary(_ checks: String) throws -> CISummary {
        try decode(#"{"number":1,"title":"t","state":"OPEN","url":"u","statusCheckRollup":[\#(checks)]}"#).ciSummary
    }

    @Test func statusContextUsesState() throws {
        #expect(try summary(#"{"__typename":"StatusContext","context":"ci","state":"SUCCESS"}"#) == .passed)
        #expect(try summary(#"{"__typename":"StatusContext","context":"ci","state":"FAILURE"}"#) == .failed(count: 1))
        #expect(try summary(#"{"__typename":"StatusContext","context":"ci","state":"ERROR"}"#) == .failed(count: 1))
        #expect(try summary(#"{"__typename":"StatusContext","context":"ci","state":"PENDING"}"#) == .running)
    }

    @Test func skippedAndNeutralAreNotFailures() throws {
        let checks = #"{"name":"a","status":"COMPLETED","conclusion":"SKIPPED"},{"name":"b","status":"COMPLETED","conclusion":"NEUTRAL"}"#
        #expect(try summary(checks) == .passed)
    }

    @Test func incompleteCheckWithoutConclusionIsRunning() throws {
        let checks = #"{"name":"a","status":"IN_PROGRESS","conclusion":""},{"name":"b","status":"COMPLETED","conclusion":"SUCCESS"}"#
        #expect(try summary(checks) == .running)
    }

    @Test func failureWinsOverRunning() throws {
        let checks = #"{"name":"a","status":"IN_PROGRESS"},{"name":"b","status":"COMPLETED","conclusion":"TIMED_OUT"},{"name":"c","status":"COMPLETED","conclusion":"CANCELLED"}"#
        #expect(try summary(checks) == .failed(count: 2))
    }

    @Test func checkNameFallsBackToContext() throws {
        let pr = try decode(
            #"{"number":1,"title":"t","state":"OPEN","url":"u","statusCheckRollup":[{"context":"ci/x","state":"SUCCESS"}]}"#)
        #expect(pr.checks[0].name == "ci/x")
    }
}
