import Testing
@testable import YakumoCore

@Suite struct StatusToneTests {
    @Test func statusToneMapping() {
        #expect(StatusTone(agent: .working) == .working)
        #expect(StatusTone(agent: .blocked) == .blocked)
        #expect(StatusTone(agent: .done) == .done)
        #expect(StatusTone(agent: .idle) == .idle)
        #expect(StatusTone(agent: .unknown) == .none)
        #expect([StatusTone.working, .blocked, .done, .idle, .none].filter(\.pulses) == [.working])
    }

    @Test func ciToneMapping() {
        #expect(CITone(.passed) == .passed)
        #expect(CITone(.failed(count: 2)) == .failed)
        #expect(CITone(.running) == .running)
        #expect(CITone(.none) == nil)
        #expect([CITone.passed, .failed, .running].filter(\.pulses) == [.running])
    }
}
