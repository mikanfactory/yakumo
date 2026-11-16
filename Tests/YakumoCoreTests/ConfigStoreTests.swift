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

    @Test func missingFileReturnsDefault() throws {
        let (store, _) = makeStore()
        let config = try store.load()
        #expect(config == YakumoConfig.default)
        #expect(config.defaultBaseRef == "origin/main")
        #expect(config.ghPath == "~/.local/share/mise/shims/gh")
    }

    @Test func roundTrip() throws {
        let (store, url) = makeStore()
        let config = YakumoConfig(defaultBaseRef: "origin/develop", ghPath: "/usr/local/bin/gh")
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
    }

    @Test func legacyRepositoriesKeyIsIgnored() throws {
        let (store, url) = makeStore()
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data(#"{"repositories":[{"name":"a","path":"/r/a"}],"gh_path":"/gh"}"#.utf8).write(to: url)
        #expect(try store.load() == YakumoConfig(defaultBaseRef: YakumoConfig.default.defaultBaseRef, ghPath: "/gh"))
    }

    @Test func malformedJSONThrows() throws {
        let (store, url) = makeStore()
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{not json".utf8).write(to: url)
        #expect(throws: (any Error).self) { try store.load() }
    }
}
