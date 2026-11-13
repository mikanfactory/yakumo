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
            Section("登録リポジトリ") {
                if config.repositories.isEmpty {
                    Text("なし（パネルで a を押して追加）").foregroundStyle(.secondary)
                }
                ForEach(config.repositories, id: \.path) { repository in
                    HStack {
                        VStack(alignment: .leading) {
                            Text(repository.name)
                            Text(repository.path).font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer()
                        Button("削除") { remove(repository) }
                    }
                }
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
        let edited = config
        update { $0.ghPath = edited.ghPath; $0.defaultBaseRef = edited.defaultBaseRef }
    }

    private func remove(_ repository: RepositoryDef) {
        update { $0.repositories.removeAll { $0.path == repository.path } }
    }

    // Starts from the file rather than the form, so a repository added from the panel meanwhile is not dropped.
    private func update(_ change: (inout YakumoConfig) -> Void) {
        do {
            var latest = try configStore.load()
            change(&latest)
            try configStore.save(latest)
            config = latest
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
