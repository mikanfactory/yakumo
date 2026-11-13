import SwiftUI
import YakumoCore

struct PRDetailView: View {
    private static let commentPrefixLength = 80

    let lookup: PRLookup?
    let isWorktree: Bool
    let onOpen: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            switch lookup {
            case .found(let info):
                HStack {
                    Text("#\(info.number) \(info.title)").lineLimit(1)
                    Spacer()
                    Text(info.mergeStateText).foregroundStyle(.secondary)
                }
                if !info.checks.isEmpty {
                    Text(info.checks.map { "\(Self.checkSymbol($0)) \($0.name)" }.joined(separator: "  "))
                        .lineLimit(2)
                }
                if let comment = info.comments.first {
                    Text("コメント \(comment.author): \(Self.prefix(of: comment.body))")
                        .foregroundStyle(.secondary).lineLimit(2)
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
        .frame(maxWidth: .infinity, minHeight: 60, alignment: .topLeading)
    }

    static func checkSymbol(_ check: PRCheck) -> String {
        if check.isFailure { return "✗" }
        if check.isSuccess { return "✓" }
        return "…"
    }

    static func prefix(of body: String) -> String {
        let flat = body.split(whereSeparator: \.isNewline).joined(separator: " ")
        return flat.count > commentPrefixLength ? String(flat.prefix(commentPrefixLength)) + "…" : flat
    }
}
