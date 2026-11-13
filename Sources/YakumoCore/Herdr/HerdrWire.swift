import Foundation

public enum HerdrMessage: Sendable {
    case result(id: String, json: Data)
    case error(id: String, code: String, message: String)
    case event(kind: String, data: Data)
}

public enum HerdrWire {
    private struct Request<Params: Encodable>: Encodable {
        let id: String
        let method: String
        let params: Params
    }

    private struct ErrorBody: Decodable {
        let code: String
        let message: String
    }

    // Decoding result/data into JSONValue and re-encoding keeps their raw shape
    // without Any, so callers pick the concrete type later.
    private struct Envelope: Decodable {
        let id: String?
        let event: String?
        let result: JSONValue?
        let data: JSONValue?
        let error: ErrorBody?
    }

    public static func encodeRequest(id: String, method: String, params: some Encodable) throws -> Data {
        let encoder = JSONEncoder()
        var data = try encoder.encode(Request(id: id, method: method, params: params))
        data.append(0x0A)
        return data
    }

    public static func decodeLine(_ line: Data) throws -> HerdrMessage {
        let envelope = try JSONDecoder().decode(Envelope.self, from: line)
        let encoder = JSONEncoder()
        if let kind = envelope.event {
            return .event(kind: kind, data: try encoder.encode(envelope.data ?? .object([:])))
        }
        if let id = envelope.id {
            if let error = envelope.error {
                return .error(id: id, code: error.code, message: error.message)
            }
            if let result = envelope.result {
                return .result(id: id, json: try encoder.encode(result))
            }
        }
        throw HerdrError(code: "malformed_message", message: "message has neither result, error nor event")
    }
}

enum JSONValue: Codable, Sendable {
    case null
    case bool(Bool)
    case number(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() {
            self = .null
        } else if let value = try? container.decode(Bool.self) {
            self = .bool(value)
        } else if let value = try? container.decode(Double.self) {
            self = .number(value)
        } else if let value = try? container.decode(String.self) {
            self = .string(value)
        } else if let value = try? container.decode([JSONValue].self) {
            self = .array(value)
        } else {
            self = .object(try container.decode([String: JSONValue].self))
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null: try container.encodeNil()
        case let .bool(value): try container.encode(value)
        case let .number(value): try container.encode(value)
        case let .string(value): try container.encode(value)
        case let .array(value): try container.encode(value)
        case let .object(value): try container.encode(value)
        }
    }
}
