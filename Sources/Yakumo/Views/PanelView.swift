import KeyboardShortcuts
import SwiftUI
import YakumoCore

struct PanelView: View {
    private static let messageLifetime: Duration = .seconds(5)
    private static let keyGuide = "enter 開く  n 新規  d 削除  o PR  r 更新  , 設定  esc 閉じる"
    private static let spacesLabel = "spaces"

    let model: AppModel
    let state: PanelState
    let onSubmitInput: () -> Void
    let onOpenPR: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            Divider()
            if let notice = connectionNotice {
                Text(notice).foregroundStyle(.orange).padding(.horizontal, 12).padding(.vertical, 4)
            }
            rowList
            modeArea
            if let progress = state.progress {
                Text(progress).foregroundStyle(.secondary).padding(.horizontal, 12).padding(.vertical, 4)
            }
            if let message = model.message {
                Text(message).foregroundStyle(.red).lineLimit(2).padding(.horizontal, 12).padding(.vertical, 4)
            }
            Divider()
            PRDetailView(lookup: currentLookup, isWorktree: currentCheckoutPath != nil, onOpen: onOpenPR)
                .padding(.horizontal, 12).padding(.vertical, 8)
            Divider()
            Text(Self.keyGuide).font(.caption).foregroundStyle(.secondary)
                .padding(.horizontal, 12).padding(.vertical, 6)
        }
        .font(.system(.body, design: .monospaced))
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(.regularMaterial)
        .ignoresSafeArea(.container, edges: .top)
        .task(id: model.message) {
            guard model.message != nil else { return }
            do { try await Task.sleep(for: Self.messageLifetime) } catch { return }
            model.message = nil
        }
    }

    private var header: some View {
        let summary = StatusSummary(rows: model.rows)
        return HStack(spacing: 12) {
            if summary.entries.isEmpty {
                if summary.total > 0 {
                    Text("\(summary.total) \(Self.spacesLabel)").foregroundStyle(.secondary)
                }
            } else {
                ForEach(summary.entries, id: \.tone) { entry in
                    HStack(spacing: 5) {
                        Circle().fill(PanelColors.status(entry.tone)).frame(width: 8, height: 8)
                        Text("\(entry.count) \(Self.label(for: entry.tone))")
                    }
                }
            }
            Spacer(minLength: 0)
            if let shortcut = KeyboardShortcuts.getShortcut(for: .togglePanel) {
                Text(shortcut.description)
                    .font(.system(size: 11))
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 6).padding(.vertical, 2)
                    .background(RoundedRectangle(cornerRadius: 4).fill(Color.primary.opacity(0.08)))
            }
        }
        .font(.system(size: 12))
        .monospacedDigit()
        .padding(.horizontal, 14).padding(.vertical, 8)
    }

    private static func label(for tone: StatusTone) -> String {
        switch tone {
        case .working: "working"
        case .blocked: "blocked"
        case .done: "done"
        case .idle: "idle"
        case .none: ""
        }
    }

    private var rowList: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 1) {
                    ForEach(Array(model.rows.enumerated()), id: \.element.id) { index, row in
                        VStack(alignment: .leading, spacing: 0) {
                            if case .settings = row {
                                Divider().padding(.vertical, 6)
                            }
                            RowView(
                                row: row, title: title(for: row), selected: index == model.cursor, pr: lookup(for: row))
                        }
                        // Not on RowView alone: scrollTo would stop at the divider, which carries the ForEach id too.
                        .id(row.id)
                    }
                }
                .font(.system(size: 13))
                .padding(.horizontal, 8).padding(.vertical, 6)
            }
            .onChange(of: model.cursor) { _, cursor in
                guard model.rows.indices.contains(cursor) else { return }
                proxy.scrollTo(model.rows[cursor].id)
            }
        }
    }

    @ViewBuilder
    private var modeArea: some View {
        switch state.mode {
        case .list:
            EmptyView()
        case .input(let kind):
            Divider()
            InputSheet(state: state, kind: kind, onSubmit: onSubmitInput).padding(12)
        case .confirmRemove(let confirmation):
            Divider()
            VStack(alignment: .leading, spacing: 4) {
                Text("「\(confirmation.label)」の worktree を削除します。ブランチは残ります。")
                Text("enter 削除  esc やめる").font(.caption).foregroundStyle(.secondary)
            }
            .padding(12)
        }
    }

    private var connectionNotice: String? {
        switch model.connection {
        case .connected: nil
        case .connecting: "herdr に接続中…"
        case .unavailable(let text): text
        }
    }

    private var currentCheckoutPath: String? {
        guard model.rows.indices.contains(model.cursor),
              case .space(let workspace) = model.rows[model.cursor]
        else { return nil }
        return workspace.worktree?.checkoutPath
    }

    private var currentLookup: PRLookup? {
        currentCheckoutPath.flatMap { model.pr[$0] }
    }

    private func title(for row: ListRow) -> String? {
        guard case .space(let workspace) = row else { return nil }
        return model.title(for: workspace)
    }

    private func lookup(for row: ListRow) -> PRLookup? {
        guard case .space(let workspace) = row, let path = workspace.worktree?.checkoutPath else { return nil }
        return model.pr[path]
    }
}
