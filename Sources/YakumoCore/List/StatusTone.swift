public enum StatusTone: Equatable, Sendable {
    case working, blocked, done, idle, none

    public init(agent: AgentStatus) {
        switch agent {
        case .working: self = .working
        case .blocked: self = .blocked
        case .done: self = .done
        case .idle: self = .idle
        case .unknown: self = .none
        }
    }

    public var pulses: Bool { self == .working }
}

public enum CITone: Equatable, Sendable {
    case passed, failed, running

    public init?(_ summary: CISummary) {
        switch summary {
        case .passed: self = .passed
        case .failed: self = .failed
        case .running: self = .running
        case .none: return nil
        }
    }

    public var pulses: Bool { self == .running }
}
