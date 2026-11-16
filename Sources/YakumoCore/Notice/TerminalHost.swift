import Foundation

public enum TerminalHost {
    static let clientName = "herdr"

    public static func find(psOutput: String, isApp: (Int32) -> Bool) -> Int32? {
        var parents: [Int32: Int32] = [:]
        var clients: [Int32] = []
        for line in psOutput.split(whereSeparator: \.isNewline) {
            let fields = line.split(separator: " ", maxSplits: 2, omittingEmptySubsequences: true)
            guard fields.count == 3, let pid = Int32(fields[0]), let ppid = Int32(fields[1]) else { continue }
            parents[pid] = ppid
            if (String(fields[2]) as NSString).lastPathComponent == clientName { clients.append(pid) }
        }
        for client in clients {
            var visited: Set<Int32> = []
            var current = parents[client]
            while let pid = current, pid > 1, visited.insert(pid).inserted {
                if isApp(pid) { return pid }
                current = parents[pid]
            }
        }
        return nil
    }
}
