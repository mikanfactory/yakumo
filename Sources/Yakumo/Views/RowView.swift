import SwiftUI
import YakumoCore

struct RowView: View {
    let row: ListRow
    let title: String?
    let selected: Bool
    let pr: PRLookup?

    var body: some View {
        HStack(spacing: 8) {
            content
            Spacer(minLength: 0)
        }
        .padding(.horizontal, isHeader ? 6 : 10)
        .padding(.top, isHeader ? 8 : 4)
        .padding(.bottom, isHeader ? 2 : 4)
        .background(
            RoundedRectangle(cornerRadius: 6)
                .fill(selected ? Color.accentColor.opacity(0.22) : Color.clear)
        )
    }

    private var isHeader: Bool {
        if case .repoHeader = row { return true }
        return false
    }

    @ViewBuilder
    private var content: some View {
        switch row {
        case .repoHeader(let name, let root):
            RepoAvatar(name: name, isOther: root == nil)
            Text(name).font(.system(size: 12, weight: .semibold)).foregroundStyle(.secondary)
        case .space(let workspace):
            let tone = StatusTone(agent: workspace.agentStatus)
            Pulsing(active: tone.pulses) {
                RoundedRectangle(cornerRadius: 1.5)
                    .fill(PanelColors.status(tone))
                    .frame(width: 3, height: 16)
            }
            Text(title ?? workspace.label).lineLimit(1).truncationMode(.middle)
            if case .found(let info) = pr {
                Spacer(minLength: 8)
                Text("#\(String(info.number))").font(.system(size: 12)).foregroundStyle(.secondary)
                if let ci = CITone(info.ciSummary) {
                    Pulsing(active: ci.pulses) {
                        Circle().fill(PanelColors.ci(ci)).frame(width: 8, height: 8)
                    }
                }
            }
        case .addWorktree:
            Label("Add worktree", systemImage: "plus").foregroundStyle(.secondary)
        case .addRepository:
            Label("Add repository", systemImage: "plus.circle").foregroundStyle(.secondary)
        case .settings:
            Label("Settings", systemImage: "gearshape").foregroundStyle(.secondary)
        }
    }
}

private struct RepoAvatar: View {
    let name: String
    let isOther: Bool

    var body: some View {
        Text(AvatarPalette.initial(for: name))
            .font(.system(size: 11, weight: .bold))
            .foregroundStyle(.white)
            .frame(width: 18, height: 18)
            .background(
                RoundedRectangle(cornerRadius: 5)
                    .fill(isOther ? PanelColors.otherAvatar : PanelColors.avatar(for: name))
            )
    }
}

private struct Pulsing<Content: View>: View {
    let active: Bool
    @ViewBuilder let content: Content
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var dimmed = false

    var body: some View {
        content
            .opacity(active && !reduceMotion && dimmed ? 0.35 : 1)
            .onAppear { restart() }
            .onChange(of: active) { _, _ in restart() }
            .onChange(of: reduceMotion) { _, _ in restart() }
    }

    private func restart() {
        guard active, !reduceMotion else {
            withAnimation(nil) { dimmed = false }
            return
        }
        withAnimation(.easeInOut(duration: 0.6).repeatForever(autoreverses: true)) { dimmed = true }
    }
}
