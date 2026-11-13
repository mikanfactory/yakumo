import Foundation

public enum NewWorktreeInputError: Error, Equatable {
    case ghFailed(String)
    case unexpectedOutput
}

public enum NewWorktreeInput: Equatable, Sendable {
    case auto
    case branch(String)
    case pullRequest(URL)

    public static func parse(_ text: String) -> NewWorktreeInput {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { return .auto }
        if let url = URL(string: trimmed), let scheme = url.scheme,
           ["http", "https"].contains(scheme), url.host != nil
        {
            return .pullRequest(url)
        }
        return .branch(trimmed)
    }

    public func resolveBranch(ghPath: String, runner: CommandRunner) async throws -> String? {
        switch self {
        case .auto:
            return nil
        case .branch(let name):
            return name
        case .pullRequest(let url):
            let output = try await runner.run(
                (ghPath as NSString).expandingTildeInPath,
                ["pr", "view", url.absoluteString, "--json", "headRefName"],
                NSHomeDirectory())
            guard output.status == 0 else {
                throw NewWorktreeInputError.ghFailed(PRFetcher.firstLine(output.stderr))
            }
            struct Head: Decodable { let headRefName: String }
            guard let head = try? JSONDecoder().decode(Head.self, from: output.stdout),
                  !head.headRefName.isEmpty
            else {
                throw NewWorktreeInputError.unexpectedOutput
            }
            return head.headRefName
        }
    }
}
