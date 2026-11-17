public struct StatusSummary: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        public let tone: StatusTone
        public let count: Int

        public init(tone: StatusTone, count: Int) {
            self.tone = tone
            self.count = count
        }
    }

    private static let reportedTones: [StatusTone] = [.working, .blocked, .done]

    public let entries: [Entry]
    public let total: Int

    public init(rows: [ListRow]) {
        let tones = rows.compactMap { row -> StatusTone? in
            guard case .space(let workspace) = row else { return nil }
            return StatusTone(agent: workspace.agentStatus)
        }
        entries = Self.reportedTones.compactMap { tone in
            let count = tones.count { $0 == tone }
            return count > 0 ? Entry(tone: tone, count: count) : nil
        }
        total = tones.count
    }
}
