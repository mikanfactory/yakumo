import SwiftUI
import YakumoCore

struct RowView: View {
    let row: ListRow
    let selected: Bool
    let pr: PRLookup?

    var body: some View {
        HStack(spacing: 6) {
            Text(selected ? ">" : " ").foregroundStyle(Color.accentColor)
            content
            Spacer(minLength: 0)
        }
        .padding(.vertical, 1)
        .background(selected ? Color.accentColor.opacity(0.15) : Color.clear)
    }

    @ViewBuilder
    private var content: some View {
        switch row {
        case .repoHeader(let name, _):
            Text(name).bold()
        case .space(let workspace):
            Text(Self.symbol(for: workspace.agentStatus))
            Text(workspace.label).lineLimit(1)
            if case .found(let info) = pr {
                Spacer()
                Text("#\(info.number) \(Self.ciText(info.ciSummary))").foregroundStyle(.secondary)
            }
        case .addWorktree:
            Text("+ Add worktree").foregroundStyle(.secondary)
        case .addRepository:
            Text("+ Add repository").foregroundStyle(.secondary)
        case .settings:
            Text("Settings").foregroundStyle(.secondary)
        }
    }

    static func symbol(for status: AgentStatus) -> String {
        switch status {
        case .idle: "○"
        case .working: "●"
        case .blocked: "◐"
        case .done: "✓"
        case .unknown: "·"
        }
    }

    static func ciText(_ summary: CISummary) -> String {
        switch summary {
        case .passed: "✓"
        case .failed(let count): "✗ \(count)"
        case .running: "…"
        case .none: ""
        }
    }
}
