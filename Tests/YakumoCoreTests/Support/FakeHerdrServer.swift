import Foundation
import os

struct ReceivedRequest: Sendable {
    let id: String
    let method: String
    let line: Data
}

struct FakeHerdrServer: Sendable {
    enum Reply: Sendable {
        case result(String)
        case error(code: String, message: String)
        case silence
        case hangUp
    }

    private struct RequestHead: Decodable {
        let id: String
        let method: String
    }

    private struct State {
        var listener: DispatchSourceRead?
        var clients: [Int32: DispatchSourceRead] = [:]
        var buffers: [Int32: Data] = [:]
        var subscribers: Set<Int32> = []
        var replies: [String: Reply] = ["events.subscribe": .result(#"{"type":"subscription_started"}"#)]
        var requests: [ReceivedRequest] = []
        var acceptedCount = 0
        var rejectsConnections = false
    }

    let path: String
    private let state = OSAllocatedUnfairLock(uncheckedState: State())
    private let queue = DispatchQueue(label: "FakeHerdrServer")

    init() {
        path = "/tmp/yk-\(UUID().uuidString.prefix(8)).sock"
    }

    static func started() throws -> FakeHerdrServer {
        let server = FakeHerdrServer()
        try server.listen()
        return server
    }

    var requests: [ReceivedRequest] { state.withLock { $0.requests } }
    var subscriberCount: Int { state.withLock { $0.subscribers.count } }
    var acceptedCount: Int { state.withLock { $0.acceptedCount } }

    func requests(method: String) -> [ReceivedRequest] {
        requests.filter { $0.method == method }
    }

    func setReply(_ reply: Reply, for method: String) {
        state.withLock { $0.replies[method] = reply }
    }

    func setRejectsConnections(_ rejects: Bool) {
        state.withLock { $0.rejectsConnections = rejects }
    }

    func sendEvent(_ line: String) {
        let data = Data((line + "\n").utf8)
        queue.sync {
            for fd in state.withLock({ $0.subscribers }) {
                writeAll(fd, data)
            }
        }
    }

    func listen() throws {
        unlink(path)
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw POSIXError(.init(rawValue: errno) ?? .EIO) }
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: Array(path.utf8)) }
        let length = socklen_t(MemoryLayout<sockaddr_un>.size)
        let bound = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(fd, $0, length) }
        }
        guard bound == 0, Darwin.listen(fd, 16) == 0 else {
            let code = errno
            close(fd)
            throw POSIXError(.init(rawValue: code) ?? .EIO)
        }
        _ = fcntl(fd, F_SETFL, O_NONBLOCK)
        let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: queue)
        source.setEventHandler { accept(on: fd) }
        source.setCancelHandler { close(fd) }
        state.withLock { $0.listener = source }
        source.resume()
    }

    func stop() {
        queue.sync {
            let sources = state.withLock { state in
                let sources = [state.listener].compactMap { $0 } + Array(state.clients.values)
                state.listener = nil
                state.clients = [:]
                state.buffers = [:]
                state.subscribers = []
                return sources
            }
            sources.forEach { $0.cancel() }
        }
        unlink(path)
    }

    func dropSubscribers() {
        queue.sync {
            let fds = state.withLock { $0.subscribers }
            fds.forEach(disconnect)
        }
    }

    private func accept(on listenFD: Int32) {
        let fd = Darwin.accept(listenFD, nil, nil)
        guard fd >= 0 else { return }
        var on: Int32 = 1
        setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &on, socklen_t(MemoryLayout<Int32>.size))
        let rejects = state.withLock { state in
            state.acceptedCount += 1
            return state.rejectsConnections
        }
        if rejects {
            close(fd)
            return
        }
        let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: queue)
        source.setEventHandler { receive(on: fd) }
        source.setCancelHandler { close(fd) }
        state.withLock { $0.clients[fd] = source }
        source.resume()
    }

    private func receive(on fd: Int32) {
        var chunk = [UInt8](repeating: 0, count: 4096)
        let count = read(fd, &chunk, chunk.count)
        guard count > 0 else {
            disconnect(fd)
            return
        }
        var buffer = state.withLock { $0.buffers[fd] ?? Data() }
        buffer.append(contentsOf: chunk[0..<count])
        while let newline = buffer.firstIndex(of: 0x0A) {
            let line = Data(buffer[buffer.startIndex..<newline])
            buffer = Data(buffer[(newline + 1)...])
            handle(line: line, from: fd)
        }
        let remaining = buffer
        state.withLock { $0.buffers[fd] = remaining }
    }

    private func handle(line: Data, from fd: Int32) {
        guard let head = try? JSONDecoder().decode(RequestHead.self, from: line) else { return }
        let reply = state.withLock { state in
            state.requests.append(ReceivedRequest(id: head.id, method: head.method, line: line))
            return state.replies[head.method] ?? .result(#"{"type":"ok"}"#)
        }
        switch reply {
        case let .result(json):
            if head.method == "events.subscribe" {
                state.withLock { _ = $0.subscribers.insert(fd) }
            }
            writeAll(fd, Data(#"{"id":"\#(head.id)","result":\#(json)}"#.utf8 + [0x0A]))
        case let .error(code, message):
            writeAll(fd, Data(#"{"id":"\#(head.id)","error":{"code":"\#(code)","message":"\#(message)"}}"#.utf8 + [0x0A]))
        case .silence:
            break
        case .hangUp:
            disconnect(fd)
        }
    }

    private func disconnect(_ fd: Int32) {
        let source = state.withLock { state in
            state.subscribers.remove(fd)
            state.buffers[fd] = nil
            return state.clients.removeValue(forKey: fd)
        }
        source?.cancel()
    }

    private func writeAll(_ fd: Int32, _ data: Data) {
        data.withUnsafeBytes { raw in
            var offset = 0
            while offset < raw.count {
                let written = write(fd, raw.baseAddress! + offset, raw.count - offset)
                guard written > 0 else { return }
                offset += written
            }
        }
    }
}
