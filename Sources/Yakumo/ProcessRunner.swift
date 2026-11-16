import Foundation
import os
import YakumoCore

enum ProcessRunner {
    static let searchPath = "/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"

    static func commandRunner() -> CommandRunner {
        CommandRunner { executable, args, cwd in
            try await withCheckedThrowingContinuation { continuation in
                DispatchQueue.global().async {
                    continuation.resume(with: Result { try run(executable, args, cwd: cwd) })
                }
            }
        }
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
