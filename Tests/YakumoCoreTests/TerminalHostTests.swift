import Testing
@testable import YakumoCore

private let psOutput = """
    1     0 /sbin/launchd
  8205     1 /Users/me/.local/bin/herdr-core
 21678     1 /Applications/Alacritty.app/Contents/MacOS/alacritty
 21686 21678 /usr/bin/login
 21687 21686 -zsh
 30743 21687 herdr
 30744 30743 /Users/me/.local/bin/herdr
 40000     1 /Applications/Ghostty.app/Contents/MacOS/ghostty
"""

@Suite struct TerminalHostTests {
    @Test func walksFromHerdrClientToItsApp() {
        let apps: Set<Int32> = [21678, 40000, 1]
        #expect(TerminalHost.find(psOutput: psOutput, isApp: { apps.contains($0) }) == 21678)
    }

    @Test func ignoresTheHerdrServer() {
        let output = """
          8205     1 /Users/me/.local/bin/herdr-core
          9000  8205 /Applications/Terminal.app/Contents/MacOS/Terminal
        """
        #expect(TerminalHost.find(psOutput: output, isApp: { $0 == 9000 }) == nil)
    }

    @Test func noClientMeansNoHost() {
        #expect(TerminalHost.find(psOutput: "    1     0 /sbin/launchd", isApp: { _ in true }) == nil)
    }

    @Test func clientWithoutAppAncestorMeansNoHost() {
        #expect(TerminalHost.find(psOutput: psOutput, isApp: { _ in false }) == nil)
    }

    @Test func malformedLinesAreSkipped() {
        let output = """
        garbage
          12 x herdr
          50     1 /Applications/Alacritty.app/Contents/MacOS/alacritty
          51    50 herdr
        """
        #expect(TerminalHost.find(psOutput: output, isApp: { $0 == 50 }) == 50)
    }

    @Test func cyclicParentsStop() {
        let output = """
          60    61 herdr
          61    60 zsh
        """
        #expect(TerminalHost.find(psOutput: output, isApp: { _ in false }) == nil)
    }
}
