import AppKit
import SwiftUI
import YakumoCore

private final class NoticePanel: NSPanel {
    override var canBecomeKey: Bool { false }
}

@MainActor
final class NoticeController {
    private static let limit = 3
    private static let lifetime: Duration = .seconds(8)
    private static let screenInset: CGFloat = 4
    private static let psPath = "/bin/ps"

    private let runner: CommandRunner
    private var model: AppModel?
    private var stack = NoticeStack(limit: NoticeController.limit)
    private var panels: [String: NoticePanel] = [:]
    private var expiries: [String: Task<Void, Never>] = [:]

    init(runner: CommandRunner) {
        self.runner = runner
    }

    func attach(_ model: AppModel) {
        self.model = model
        model.onNotice = { [weak self] in self?.show($0) }
    }

    func show(_ notice: AgentNotice) {
        for dropped in stack.push(notice) { close(id: dropped) }
        close(id: notice.id)
        let panel = makePanel(for: notice)
        panels[notice.id] = panel
        layout(newest: notice.id)
        panel.orderFrontRegardless()
        scheduleExpiry(for: notice.id)
    }

    private func makePanel(for notice: AgentNotice) -> NoticePanel {
        let id = notice.id
        let view = NoticeCardView(
            notice: notice,
            onOpen: { [weak self] in self?.open(notice) },
            onClose: { [weak self] in self?.dismiss(id: id) },
            onHover: { [weak self] inside in self?.hoverChanged(id: id, inside: inside) }
        )
        let hosting = NSHostingView(rootView: view)
        let panel = NoticePanel(
            contentRect: NSRect(origin: .zero, size: hosting.fittingSize),
            styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
        panel.contentView = hosting
        panel.isFloatingPanel = true
        panel.level = .statusBar
        panel.backgroundColor = .clear
        panel.isOpaque = false
        panel.hasShadow = true
        panel.hidesOnDeactivate = false
        panel.isReleasedWhenClosed = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary]
        return panel
    }

    // Not animating the newest card: it appears in place, and only the older ones slide down to make room.
    private func layout(newest: String? = nil) {
        guard let screen = NSScreen.main else { return }
        let area = screen.visibleFrame
        var top = area.maxY - Self.screenInset
        for notice in stack.notices {
            guard let panel = panels[notice.id] else { continue }
            let size = panel.frame.size
            let origin = NSPoint(x: area.maxX - Self.screenInset - size.width, y: top - size.height)
            panel.setFrame(NSRect(origin: origin, size: size), display: true, animate: notice.id != newest)
            top -= size.height
        }
    }

    private func scheduleExpiry(for id: String) {
        expiries[id]?.cancel()
        expiries[id] = Task { [weak self] in
            do { try await Task.sleep(for: Self.lifetime) } catch { return }
            self?.dismiss(id: id)
        }
    }

    private func hoverChanged(id: String, inside: Bool) {
        if inside {
            expiries[id]?.cancel()
            expiries[id] = nil
        } else {
            scheduleExpiry(for: id)
        }
    }

    private func dismiss(id: String) {
        stack.remove(id: id)
        close(id: id)
        layout()
    }

    private func close(id: String) {
        expiries.removeValue(forKey: id)?.cancel()
        panels.removeValue(forKey: id)?.orderOut(nil)
    }

    private func open(_ notice: AgentNotice) {
        dismiss(id: notice.id)
        guard let model else { return }
        Task {
            guard await model.openNotice(notice) else { return }
            await activateTerminal()
        }
    }

    private func activateTerminal() async {
        let output = try? await runner.run(Self.psPath, ["-axo", "pid=,ppid=,comm="], NSHomeDirectory())
        guard let output, output.status == 0 else { return }
        let ps = String(decoding: output.stdout, as: UTF8.self)
        let host = TerminalHost.find(psOutput: ps) { NSRunningApplication(processIdentifier: $0) != nil }
        guard let host, let app = NSRunningApplication(processIdentifier: host) else { return }
        app.activate()
    }
}
