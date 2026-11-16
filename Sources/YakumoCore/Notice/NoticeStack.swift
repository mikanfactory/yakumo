public struct NoticeStack: Sendable {
    public let limit: Int
    public private(set) var notices: [AgentNotice] = []

    public init(limit: Int) {
        self.limit = limit
    }

    public mutating func push(_ notice: AgentNotice) -> [String] {
        notices.removeAll { $0.id == notice.id }
        notices.insert(notice, at: 0)
        let overflow = notices.dropFirst(limit).map(\.id)
        notices = Array(notices.prefix(limit))
        return overflow
    }

    public mutating func remove(id: String) {
        notices.removeAll { $0.id == id }
    }
}
