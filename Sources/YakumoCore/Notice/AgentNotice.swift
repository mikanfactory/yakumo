public struct AgentNotice: Equatable, Identifiable, Sendable {
    public let paneID: String
    public let workspaceID: String
    public let name: String
    public let detail: String?
    public let isRepository: Bool
    public let status: AgentStatus

    public init(paneID: String, workspaceID: String, name: String, detail: String?, isRepository: Bool, status: AgentStatus) {
        self.paneID = paneID
        self.workspaceID = workspaceID
        self.name = name
        self.detail = detail
        self.isRepository = isRepository
        self.status = status
    }

    public var id: String { paneID }

    public var statusText: String {
        status == .blocked ? "入力待ち" : "完了"
    }

    static let raisingStatuses: Set<AgentStatus> = [.done, .blocked]
}
