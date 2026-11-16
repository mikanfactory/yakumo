import SwiftUI
import YakumoCore

struct PRDetailView: View {
    let lookup: PRLookup?
    let isWorktree: Bool
    let onOpen: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            switch lookup {
            case .found(let info):
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text("#\(String(info.number))").foregroundStyle(.secondary).monospacedDigit()
                    Text(info.title).fixedSize(horizontal: false, vertical: true)
                }
                Button("PR を開く", action: onOpen)
            case .some(.none):
                Text("PR なし").foregroundStyle(.secondary)
            case .failed(let line):
                Text(line).foregroundStyle(.red).lineLimit(1)
            case nil:
                Text(isWorktree ? "PR を取得中…" : " ").foregroundStyle(.secondary)
            }
        }
        .font(.system(size: 13))
        .frame(maxWidth: .infinity, minHeight: 60, alignment: .topLeading)
    }
}
