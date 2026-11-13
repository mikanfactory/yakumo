import KeyboardShortcuts
import SwiftUI
import YakumoCore

struct PanelView: View {
    private static let messageLifetime: Duration = .seconds(5)
    private static let keyGuide = "enter 開く  n 新規  d 削除  o PR  r 更新  a 追加  , 設定  esc 閉じる"

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
        .task(id: model.message) {
            guard model.message != nil else { return }
            do { try await Task.sleep(for: Self.messageLifetime) } catch { return }
            model.message = nil
        }
    }

    private var header: some View {
        HStack {
            Text("yakumo").bold()
            Spacer()
            Text(KeyboardShortcuts.getShortcut(for: .togglePanel)?.description ?? "").foregroundStyle(.secondary)
        }
        .padding(.horizontal, 12).padding(.top, 10).padding(.bottom, 6)
    }

    private var rowList: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 2) {
                    ForEach(Array(model.rows.enumerated()), id: \.element.id) { index, row in
                        RowView(row: row, selected: index == model.cursor, pr: lookup(for: row)).id(row.id)
                    }
                }
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

    private func lookup(for row: ListRow) -> PRLookup? {
        guard case .space(let workspace) = row, let path = workspace.worktree?.checkoutPath else { return nil }
        return model.pr[path]
    }
}
