import AppKit
import SwiftUI
import YakumoCore

enum PanelColors {
    static let avatars: [Color] = [.blue, .purple, .green, .orange, .pink, .teal, .indigo, .red]
    static let otherAvatar = Color(nsColor: .systemGray)

    static func avatar(for name: String) -> Color {
        avatars[AvatarPalette.index(for: name) % avatars.count]
    }

    static func status(_ tone: StatusTone) -> Color {
        switch tone {
        case .working: .orange
        case .blocked: .red
        case .done: .green
        case .idle: Color(nsColor: .systemGray)
        case .none: Color(nsColor: .quaternaryLabelColor)
        }
    }

    static func ci(_ tone: CITone) -> Color {
        switch tone {
        case .passed: .green
        case .failed: .red
        case .running: .orange
        }
    }
}
