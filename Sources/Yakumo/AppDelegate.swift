import AppKit
import KeyboardShortcuts
import SwiftUI
import YakumoCore

extension KeyboardShortcuts.Name {
    nonisolated(unsafe) static let togglePanel = Self("togglePanel", default: .init(.g, modifiers: [.command]))
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private static let configURL = URL(fileURLWithPath: NSHomeDirectory())
        .appendingPathComponent(".config/yakumo/config.json")
    private static let herdrSocketPath = NSHomeDirectory() + "/.config/herdr/herdr.sock"

    private let configStore = ConfigStore(url: AppDelegate.configURL)
    private let runner = ProcessRunner.commandRunner()
    private var statusItem: NSStatusItem?
    private var panel: PanelController?
    private lazy var notices = NoticeController(runner: runner)
    private var startTask: Task<Void, Never>?
    private var settingsWindow: NSWindow?

    func applicationDidFinishLaunching(_ notification: Notification) {
        let model = makeModel()
        notices.attach(model)
        panel = PanelController(model: model) { [weak self] in self?.openSettings() }
        startTask = Task { await model.start() }
        installMainMenu()
        installStatusItem()
        KeyboardShortcuts.onKeyUp(for: .togglePanel) { [weak self] in
            MainActor.assumeIsolated { self?.panel?.toggle() }
        }
    }

    private func makeModel() -> AppModel {
        // Not left to AppModel.start(): PRFetcher takes gh_path only at init, so the config is read here first.
        let config = (try? configStore.load()) ?? .default
        let prFetcher = PRFetcher(ghPath: config.ghPath, runner: runner, now: { ContinuousClock.now })
        return AppModel(
            herdr: HerdrClient(socketPath: Self.herdrSocketPath),
            configStore: configStore,
            prFetcher: prFetcher,
            runner: runner
        )
    }

    private func rebuildModel() {
        startTask?.cancel()
        let model = makeModel()
        notices.attach(model)
        panel?.replaceModel(model)
        startTask = Task { await model.start() }
    }

    // Not skipped for an accessory app: the menu bar stays hidden, but without a main menu
    // Cmd-C/V/X/A/Z never reach the text fields.
    private func installMainMenu() {
        let editMenu = NSMenu(title: "Edit")
        editMenu.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        editMenu.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "Z")
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        editMenu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

        let appMenu = NSMenu(title: "Yakumo")
        appMenu.addItem(withTitle: "Quit Yakumo", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

        let mainMenu = NSMenu()
        for submenu in [appMenu, editMenu] {
            let item = NSMenuItem(title: submenu.title, action: nil, keyEquivalent: "")
            item.submenu = submenu
            mainMenu.addItem(item)
        }
        NSApp.mainMenu = mainMenu
    }

    private func installStatusItem() {
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        item.button?.image = NSImage(systemSymbolName: "square.stack.3d.up", accessibilityDescription: "Yakumo")
        let menu = NSMenu()
        menu.addItem(NSMenuItem(title: "パネルを開く", action: #selector(showPanel), keyEquivalent: ""))
        menu.addItem(NSMenuItem(title: "Settings…", action: #selector(showSettings), keyEquivalent: ","))
        menu.addItem(.separator())
        menu.addItem(NSMenuItem(title: "終了", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))
        for menuItem in menu.items where menuItem.action != #selector(NSApplication.terminate(_:)) {
            menuItem.target = self
        }
        item.menu = menu
        statusItem = item
    }

    @objc private func showPanel() {
        panel?.show()
    }

    @objc private func showSettings() {
        openSettings()
    }

    private func openSettings() {
        let window = settingsWindow ?? makeSettingsWindow()
        // Not reusing the previous view: it would keep the unsaved edits and the status line from the last open.
        let controller = NSHostingController(
            rootView: SettingsView(configStore: configStore) { [weak self] in self?.rebuildModel() })
        window.contentViewController = controller
        window.setContentSize(controller.view.fittingSize)
        settingsWindow = window
        window.center()
        window.makeKeyAndOrderFront(nil)
    }

    // Not a regular window: the terminal behind the non-activating panel never yields activation,
    // so a regular window opens behind it.
    private func makeSettingsWindow() -> NSWindow {
        let window = KeyablePanel(
            contentRect: .zero, styleMask: [.titled, .closable, .nonactivatingPanel],
            backing: .buffered, defer: false)
        window.title = "Yakumo Settings"
        window.isReleasedWhenClosed = false
        window.isFloatingPanel = true
        window.level = .floating
        window.hidesOnDeactivate = false
        window.collectionBehavior = [.moveToActiveSpace, .fullScreenAuxiliary]
        return window
    }
}
