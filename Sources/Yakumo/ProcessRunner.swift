import Foundation
import os
import YakumoCore

struct CommandFailedError: Error, CustomStringConvertible {
    let command: String
    let status: Int32
    let stderr: String

    var description: String {
        let line = stderr.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
        return line.isEmpty ? "\(command) が終了コード \(status) で失敗しました" : line
    }
}

enum ProcessRunner {
    static let searchPath = "/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"
    private static let gitPath = "/usr/bin/git"

    static func commandRunner() -> CommandRunner {
        CommandRunner { executable, args, cwd in
            try await withCheckedThrowingContinuation { continuation in
                DispatchQueue.global().async {
                    continuation.resume(with: Result { try run(executable, args, cwd: cwd) })
                }
            }
        }
    }

    @Sendable
    static func gitToplevel(_ path: String) throws -> String {
        let output = try run(gitPath, ["-C", path, "rev-parse", "--show-toplevel"], cwd: NSHomeDirectory())
        guard output.status == 0 else {
            throw CommandFailedError(command: "git", status: output.status, stderr: output.stderr)
        }
        return String(decoding: output.stdout, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func run(_ executable: String, _ args: [String], cwd: String) throws
        -> (status: Int32, stdout: Data, stderr: String)
    {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = args
        process.currentDirectoryURL = URL(fileURLWithPath: cwd)
        process.environment = ["PATH": searchPath, "HOME": NSHomeDirectory()]
        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr
        process.standardInput = FileHandle.nullDevice
        try process.run()

        // Not read one after the other: a child that fills the stderr pipe would block before closing stdout.
        let stderrData = OSAllocatedUnfairLock(initialState: Data())
        let group = DispatchGroup()
        group.enter()
        DispatchQueue.global().async {
            let data = stderr.fileHandleForReading.readDataToEndOfFile()
            stderrData.withLock { $0 = data }
            group.leave()
        }
        let stdoutData = stdout.fileHandleForReading.readDataToEndOfFile()
        group.wait()
        process.waitUntilExit()
        let errorText = String(decoding: stderrData.withLock { $0 }, as: UTF8.self)
        return (process.terminationStatus, stdoutData, errorText)
    }
}
