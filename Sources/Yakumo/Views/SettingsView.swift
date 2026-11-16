import KeyboardShortcuts
import ServiceManagement
import SwiftUI
import YakumoCore

struct SettingsView: View {
    let configStore: ConfigStore
    let onSaved: () -> Void

    @State private var config: YakumoConfig = .default
    @State private var loginItemEnabled = SMAppService.mainApp.status == .enabled
    @State private var status: String?

    var body: some View {
        Form {
            Section("ホットキー") {
                KeyboardShortcuts.Recorder("パネルの開閉", name: .togglePanel)
            }
            Section("起動") {
                Toggle("ログイン時に起動", isOn: Binding(get: { loginItemEnabled }, set: { setLoginItem($0) }))
            }
            Section("worktree と PR") {
                TextField("gh のパス", text: $config.ghPath)
                TextField("default_base_ref", text: $config.defaultBaseRef)
                Button("保存", action: saveFields)
            }
            if let status {
                Text(status).foregroundStyle(.secondary)
            }
        }
        .formStyle(.grouped)
        .frame(width: 520, height: 520)
        .onAppear(perform: load)
    }

    private func load() {
        do {
            config = try configStore.load()
        } catch {
            status = "設定ファイルを読めません: \(error)"
        }
    }

    private func saveFields() {
        do {
            try configStore.save(config)
            status = "保存しました"
            onSaved()
        } catch {
            status = "保存できません: \(error)"
        }
    }

    private func setLoginItem(_ enabled: Bool) {
        do {
            if enabled {
                try SMAppService.mainApp.register()
            } else {
                try SMAppService.mainApp.unregister()
            }
            status = SMAppService.mainApp.status == .requiresApproval
                ? "システム設定の「ログイン項目」で Yakumo を許可してください" : nil
        } catch {
            status = "ログイン項目を変更できません: \(error.localizedDescription)"
        }
        loginItemEnabled = SMAppService.mainApp.status == .enabled
    }
}
