import Foundation
import Testing
@testable import YakumoCore

struct ConfigStoreTests {
    private func makeStore() -> (ConfigStore, URL) {
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("yakumo-config-\(UUID().uuidString)", isDirectory: true)
        let url = dir.appendingPathComponent("nested/config.json")
        return (ConfigStore(url: url), url)
    }

    private struct NotGit: Error, Equatable {}

    @Test func missingFileReturnsDefault() throws {
        let (store, _) = makeStore()
        let config = try store.load()
        #expect(config == YakumoConfig.default)
        #expect(config.defaultBaseRef == "origin/main")
        #expect(config.ghPath == "/opt/homebrew/bin/gh")
        #expect(config.repositories.isEmpty)
    }

    @Test func roundTrip() throws {
        let (store, url) = makeStore()
        let config = YakumoConfig(
            repositories: [RepositoryDef(name: "a", path: "/r/a")],
            defaultBaseRef: "origin/develop",
            ghPath: "/usr/local/bin/gh"
        )
        try store.save(config)
        #expect(try store.load() == config)
        let text = try String(contentsOf: url, encoding: .utf8)
        #expect(text.contains("\"default_base_ref\""))
        #expect(text.contains("\"gh_path\""))
        #expect(text.contains("\n"))
    }

    @Test func partialFileFillsDefaults() throws {
        let (store, url) = makeStore()
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data(#"{"default_base_ref":"origin/dev"}"#.utf8).write(to: url)
        let config = try store.load()
        #expect(config.defaultBaseRef == "origin/dev")
        #expect(config.ghPath == YakumoConfig.default.ghPath)
        #expect(config.repositories.isEmpty)
    }

    @Test func malformedJSONThrows() throws {
        let (store, url) = makeStore()
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{not json".utf8).write(to: url)
        #expect(throws: (any Error).self) { try store.load() }
    }

    @Test func addRepositoryNormalizesAndDedupes() throws {
        let (store, _) = makeStore()
        var received: [String] = []
        let toplevel: (String) throws -> String = { path in
            received.append(path)
            return "/r/a"
        }
        _ = try store.addRepository(path: "~/x", gitToplevel: toplevel)
        let config = try store.addRepository(path: "/r/a/sub", gitToplevel: toplevel)

        let home = FileManager.default.homeDirectoryForCurrentUser.path
        #expect(received == ["\(home)/x", "/r/a/sub"])
        #expect(config.repositories == [RepositoryDef(name: "a", path: "/r/a")])
        #expect(try store.load() == config)
    }

    @Test func addRepositoryRejectsNonGit() throws {
        let (store, _) = makeStore()
        #expect(throws: NotGit.self) {
            try store.addRepository(path: "/not/git") { _ in throw NotGit() }
        }
        #expect(try store.load() == YakumoConfig.default)
    }
}
