import SwiftUI
import YakumoCore

struct NoticeCardView: View {
    static let width: CGFloat = 320

    let notice: AgentNotice
    let onOpen: () -> Void
    let onClose: () -> Void
    let onHover: (Bool) -> Void
    @State private var hovering = false

    private var tint: Color { PanelColors.status(StatusTone(agent: notice.status)) }

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            RoundedRectangle(cornerRadius: 1.5)
                .fill(tint)
                .frame(width: 3)
            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    RepoAvatar(name: notice.name, isOther: !notice.isRepository)
                    Text(notice.name).font(.system(size: 12, weight: .semibold)).foregroundStyle(.secondary)
                        .lineLimit(1).truncationMode(.middle)
                    Spacer(minLength: 4)
                    Text(notice.statusText).font(.system(size: 12, weight: .medium))
                        .foregroundStyle(tint)
                }
                if let detail = notice.detail {
                    Text(detail).font(.system(size: 13)).lineLimit(1).truncationMode(.middle)
                }
            }
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 12)
        .frame(width: Self.width, alignment: .leading)
        .fixedSize(horizontal: false, vertical: true)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(.separator, lineWidth: 0.5))
        .overlay(alignment: .topLeading) {
            if hovering {
                Button(action: onClose) {
                    Image(systemName: "xmark.circle.fill").font(.system(size: 14)).foregroundStyle(.secondary)
                        .background(Circle().fill(.background))
                }
                .buttonStyle(.plain)
                .offset(x: -6, y: -6)
            }
        }
        .contentShape(RoundedRectangle(cornerRadius: 10))
        .onTapGesture(perform: onOpen)
        .onHover { inside in
            hovering = inside
            onHover(inside)
        }
        .padding(8)
    }
}
