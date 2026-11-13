import Foundation

struct UnixSocket: Sendable {
    let fd: Int32
    private var buffer = Data()

    private init(fd: Int32) {
        self.fd = fd
    }

    static func connect(path: String, timeout: Duration?) throws -> UnixSocket {
        var address = sockaddr_un()
        let pathBytes = Array(path.utf8)
        guard pathBytes.count < MemoryLayout.size(ofValue: address.sun_path) else {
            throw HerdrError(code: "connection_failed", message: "socket path is too long: \(path)")
        }
        address.sun_family = sa_family_t(AF_UNIX)
        withUnsafeMutableBytes(of: &address.sun_path) { $0.copyBytes(from: pathBytes) }

        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { throw posixError("connection_failed", "socket") }
        var on: Int32 = 1
        setsockopt(fd, SOL_SOCKET, SO_NOSIGPIPE, &on, socklen_t(MemoryLayout<Int32>.size))
        if let timeout {
            var interval = timeval(duration: timeout)
            let size = socklen_t(MemoryLayout<timeval>.size)
            setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &interval, size)
            setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &interval, size)
        }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        guard connected == 0 else {
            let error = posixError("connection_failed", "connect \(path)")
            Darwin.close(fd)
            throw error
        }
        return UnixSocket(fd: fd)
    }

    func write(_ data: Data) throws {
        try data.withUnsafeBytes { raw in
            guard let base = raw.baseAddress else { return }
            var offset = 0
            while offset < raw.count {
                let written = Darwin.write(fd, base + offset, raw.count - offset)
                if written < 0 {
                    if errno == EINTR { continue }
                    throw Self.ioError("write")
                }
                offset += written
            }
        }
    }

    mutating func readLine() throws -> Data? {
        var chunk = [UInt8](repeating: 0, count: 4096)
        while true {
            if let newline = buffer.firstIndex(of: 0x0A) {
                let line = Data(buffer[buffer.startIndex..<newline])
                buffer = Data(buffer[(newline + 1)...])
                return line
            }
            let count = Darwin.read(fd, &chunk, chunk.count)
            if count == 0 { return nil }
            if count < 0 {
                if errno == EINTR { continue }
                throw Self.ioError("read")
            }
            buffer.append(contentsOf: chunk[0..<count])
        }
    }

    func shutdown() {
        _ = Darwin.shutdown(fd, SHUT_RDWR)
    }

    func close() {
        Darwin.close(fd)
    }

    private static func ioError(_ operation: String) -> HerdrError {
        if errno == EAGAIN || errno == EWOULDBLOCK {
            return HerdrError(code: "timeout", message: "herdr did not answer in time")
        }
        return posixError("io_error", operation)
    }

    private static func posixError(_ code: String, _ operation: String) -> HerdrError {
        HerdrError(code: code, message: "\(operation): \(String(cString: strerror(errno)))")
    }
}

private extension timeval {
    init(duration: Duration) {
        let (seconds, attoseconds) = duration.components
        self.init(tv_sec: Int(seconds), tv_usec: Int32(attoseconds / 1_000_000_000_000))
    }
}
