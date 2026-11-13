import Foundation
import Testing
@testable import YakumoCore

struct PathCompleterTests {
    private func makeHome(dirs: [String], files: [String] = []) throws -> String {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("pathcompleter-\(UUID().uuidString)")
        for dir in dirs {
            try FileManager.default.createDirectory(
                at: root.appendingPathComponent(dir), withIntermediateDirectories: true)
        }
        for file in files {
            try Data().write(to: root.appendingPathComponent(file))
        }
        return root.path
    }

    @Test func expandsTildeAndKeepsIt() throws {
        let home = try makeHome(dirs: ["code/alpha", "code/beta"])
        #expect(PathCompleter.complete("~/code/", home: home) == ["~/code/alpha/", "~/code/beta/"])
        #expect(PathCompleter.complete("~/code/a", home: home) == ["~/code/alpha/"])
        #expect(PathCompleter.complete("~", home: home) == ["~/code/"])
    }

    @Test func absolutePathsAreReturnedAsTyped() throws {
        let home = try makeHome(dirs: ["x/one"])
        #expect(PathCompleter.complete("\(home)/x/", home: "/nonexistent") == ["\(home)/x/one/"])
    }

    @Test func directoriesOnly() throws {
        let home = try makeHome(dirs: ["d1"], files: ["f1"])
        try FileManager.default.createSymbolicLink(
            atPath: home + "/link", withDestinationPath: home + "/d1")
        try FileManager.default.createSymbolicLink(
            atPath: home + "/filelink", withDestinationPath: home + "/f1")
        #expect(PathCompleter.complete("~/", home: home) == ["~/d1/", "~/link/"])
    }

    @Test func capsAt50() throws {
        let names = (0..<60).map { String(format: "d%02d", $0) }
        let home = try makeHome(dirs: names)
        let result = PathCompleter.complete("~/", home: home)
        #expect(result.count == 50)
        #expect(result.first == "~/d00/")
        #expect(result.last == "~/d49/")
    }

    @Test func hiddenOnlyWhenDotTyped() throws {
        let home = try makeHome(dirs: [".config", ".cache", "code"])
        #expect(PathCompleter.complete("~/", home: home) == ["~/code/"])
        #expect(PathCompleter.complete("~/.", home: home) == ["~/.cache/", "~/.config/"])
        #expect(PathCompleter.complete("~/.c", home: home) == ["~/.cache/", "~/.config/"])
    }

    @Test func nonexistentParentReturnsEmpty() throws {
        let home = try makeHome(dirs: [])
        #expect(PathCompleter.complete("~/missing/x", home: home).isEmpty)
    }

    @Test func emptyAndSlashlessInputReturnEmpty() throws {
        let home = try makeHome(dirs: ["foo"])
        #expect(PathCompleter.complete("", home: home).isEmpty)
        #expect(PathCompleter.complete("foo", home: home).isEmpty)
    }

    @Test func prefixMatchIsCaseSensitive() throws {
        let home = try makeHome(dirs: ["Alpha"])
        #expect(PathCompleter.complete("~/a", home: home).isEmpty)
        #expect(PathCompleter.complete("~/A", home: home) == ["~/Alpha/"])
    }
}
