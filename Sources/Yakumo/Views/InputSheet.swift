import SwiftUI

struct InputSheet: View {
    @Bindable var state: PanelState
    let kind: InputKind
    let onSubmit: () -> Void
    @FocusState private var focused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title)
            TextField(placeholder, text: $state.inputText)
                .textFieldStyle(.roundedBorder)
                .focused($focused)
                .onSubmit(onSubmit)
            Text("enter 作成  esc やめる")
                .font(.caption).foregroundStyle(.secondary)
        }
        .onAppear { focused = true }
    }

    private var title: String {
        switch kind {
        case .newWorktree(let repoRoot): "新しい worktree（\((repoRoot as NSString).lastPathComponent)）"
        }
    }

    private var placeholder: String {
        switch kind {
        case .newWorktree: "空欄 / ブランチ名 / PR の URL"
        }
    }
}
