import Foundation

public enum Cursor {
    public static func move(_ index: Int, by delta: Int, in rows: [ListRow]) -> Int {
        guard delta != 0 else { return index }
        let step = delta > 0 ? 1 : -1
        var current = index
        for _ in 0..<abs(delta) {
            var next = current + step
            while rows.indices.contains(next), !rows[next].isSelectable { next += step }
            guard rows.indices.contains(next) else { break }
            current = next
        }
        return current
    }

    public static func initial(in rows: [ListRow]) -> Int {
        if let focused = rows.firstIndex(where: {
            if case .space(let workspace) = $0 { return workspace.focused }
            return false
        }) {
            return focused
        }
        return rows.firstIndex(where: \.isSelectable) ?? 0
    }
}
