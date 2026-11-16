import AppKit
import Observation
import SwiftUI
import YakumoCore

enum InputKind: Equatable {
    case newWorktree(repoRoot: String)
}

enum PanelMode: Equatable {
    case list
    case input(InputKind)
    case confirmRemove(RemoveConfirmation)
}

@MainActor
@Observable
final class PanelState {
    var mode: PanelMode = .list
    var inputText = ""
    var progress: String?

    func begin(_ kind: InputKind) {
        inputText = ""
        mode = .input(kind)
    }
}

final class KeyablePanel: NSPanel {
    override var canBecomeKey: Bool { true }
}

private enum KeyCode {
    static let returnKey: UInt16 = 36
    static let keypadEnter: UInt16 = 76
    static let escape: UInt16 = 53
    static let down: UInt16 = 125
    static let up: UInt16 = 126
}

@MainActor
final class PanelController: NSObject, NSWindowDelegate {
    // Not a regular panel by default: activating Yakumo would take the active-app status away from the terminal.
    // If keys do not reach the panel while the terminal is frontmost, set this to false.
    private static let nonActivating = true
    private static let size = NSSize(width: 560, height: 560)
    private static let creatingText = "worktree を作成中…"
    private static let removingText = "worktree を削除中…"
    private static let noRepoText = "リポジトリの行で n を押してください"
    private static let noPRText = "この行には PR がありません"

    private var model: AppModel
    private let state = PanelState()
    private let panel: KeyablePanel
    private let hosting: NSHostingView<PanelView>
    private let openSettings: () -> Void
    private var previousApp: NSRunningApplication?
    private var keyMonitor: Any?

    init(model: AppModel, openSettings: @escaping () -> Void) {
        self.model = model
        self.openSettings = openSettings
        var style: NSWindow.StyleMask = [.titled, .fullSizeContentView]
        if Self.nonActivating { style.insert(.nonactivatingPanel) }
        panel = KeyablePanel(
            contentRect: NSRect(origin: .zero, size: Self.size), styleMask: style, backing: .buffered, defer: true)
        hosting = NSHostingView(rootView: PanelView(model: model, state: state, onSubmitInput: {}, onOpenPR: {}))
        super.init()
        panel.isFloatingPanel = true
        panel.level = .floating
        panel.hidesOnDeactivate = false
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.isMovableByWindowBackground = true
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        [.closeButton, .miniaturizeButton, .zoomButton].forEach { panel.standardWindowButton($0)?.isHidden = true }
        panel.contentView = hosting
        panel.delegate = self
        refreshRootView()
        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            let consumed = MainActor.assumeIsolated { self?.handle(event) ?? false }
            return consumed ? nil : event
        }
    }

    var isVisible: Bool { panel.isVisible }

    func toggle() {
        isVisible ? hide(restoringFocus: true) : show()
    }

    func show() {
        guard !panel.isVisible else { return }
        previousApp = NSWorkspace.shared.frontmostApplication
        state.mode = .list
        center()
        if !Self.nonActivating { NSApp.activate() }
        panel.makeKeyAndOrderFront(nil)
        model.setPanelVisible(true)
    }

    // Not restored by default: on resign key, Settings or `o` another window is taking focus,
    // and bringing the previous app back would cover it.
    func hide(restoringFocus: Bool = false) {
        guard panel.isVisible else { return }
        panel.orderOut(nil)
        state.mode = .list
        model.setPanelVisible(false)
        if restoringFocus, !Self.nonActivating, let previousApp, previousApp != NSRunningApplication.current {
            previousApp.activate(from: .current)
        }
    }

    func replaceModel(_ newModel: AppModel) {
        let visible = panel.isVisible
        model.setPanelVisible(false)
        model = newModel
        refreshRootView()
        if visible { newModel.setPanelVisible(true) }
    }

    func windowDidResignKey(_ notification: Notification) {
        hide()
    }

    private func refreshRootView() {
        hosting.rootView = PanelView(
            model: model, state: state,
            onSubmitInput: { [weak self] in self?.submitInput() },
            onOpenPR: { [weak self] in self?.openPR() })
    }

    private func center() {
        let mouse = NSEvent.mouseLocation
        let screen = NSScreen.screens.first { $0.frame.contains(mouse) } ?? NSScreen.main
        guard let visible = screen?.visibleFrame else { return }
        panel.setFrameOrigin(NSPoint(
            x: visible.midX - Self.size.width / 2, y: visible.midY - Self.size.height / 2))
    }

    private func handle(_ event: NSEvent) -> Bool {
        guard event.window === panel,
              event.modifierFlags.intersection([.command, .control, .option]).isEmpty
        else { return false }
        switch state.mode {
        case .list: return handleListKey(event)
        case .input: return handleInputKey(event)
        case .confirmRemove(let confirmation): return handleConfirmKey(event, confirmation: confirmation)
        }
    }

    private func handleListKey(_ event: NSEvent) -> Bool {
        model.message = nil
        switch event.keyCode {
        case KeyCode.escape: hide(restoringFocus: true)
        case KeyCode.returnKey, KeyCode.keypadEnter: activateRow()
        case KeyCode.down: model.moveCursor(1)
        case KeyCode.up: model.moveCursor(-1)
        default:
            switch event.charactersIgnoringModifiers {
            case "j": model.moveCursor(1)
            case "k": model.moveCursor(-1)
            case "n": beginNewWorktree()
            case "d": if let confirmation = model.requestRemove() { state.mode = .confirmRemove(confirmation) }
            case "o": openPR()
            case "r": Task { await model.refresh() }
            case ",": showSettings()
            default: return false
            }
        }
        return true
    }

    private func handleInputKey(_ event: NSEvent) -> Bool {
        // Not intercepted while the IME is composing: esc and tab belong to the conversion then.
        if let editor = panel.firstResponder as? NSTextView, editor.hasMarkedText() { return false }
        switch event.keyCode {
        case KeyCode.escape:
            state.mode = .list
            return true
        default:
            return false
        }
    }

    private func handleConfirmKey(_ event: NSEvent, confirmation: RemoveConfirmation) -> Bool {
        switch event.keyCode {
        case KeyCode.returnKey, KeyCode.keypadEnter:
            state.mode = .list
            state.progress = Self.removingText
            Task {
                await model.confirmRemove(confirmation)
                state.progress = nil
            }
        case KeyCode.escape:
            state.mode = .list
        default:
            break
        }
        return true
    }

    private var currentRow: ListRow? {
        model.rows.indices.contains(model.cursor) ? model.rows[model.cursor] : nil
    }

    private func activateRow() {
        switch currentRow {
        case .space:
            Task {
                if await model.activate() { hide(restoringFocus: true) }
            }
        case .addWorktree(let repoRoot): state.begin(.newWorktree(repoRoot: repoRoot))
        case .settings: showSettings()
        case .repoHeader, nil: break
        }
    }

    private func beginNewWorktree() {
        switch currentRow {
        case .space(let workspace) where workspace.worktree != nil:
            state.begin(.newWorktree(repoRoot: workspace.worktree?.repoRoot ?? ""))
        case .addWorktree(let repoRoot):
            state.begin(.newWorktree(repoRoot: repoRoot))
        default:
            model.message = Self.noRepoText
        }
    }

    private func submitInput() {
        guard case .input(let kind) = state.mode else { return }
        let text = state.inputText
        state.mode = .list
        switch kind {
        case .newWorktree(let repoRoot):
            state.progress = Self.creatingText
            let model = model
            Task {
                await model.createWorktree(repoRoot: repoRoot, input: text)
                state.progress = nil
            }
        }
    }

    private func openPR() {
        guard let url = model.prURLForCursor() else {
            model.message = Self.noPRText
            return
        }
        NSWorkspace.shared.open(url)
    }

    private func showSettings() {
        hide()
        openSettings()
    }
}
