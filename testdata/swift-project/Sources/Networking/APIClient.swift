import Foundation

public class APIClient {
    private let baseURL: String
    public let session: URLSession

    public init(baseURL: String, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public func get(path: String) async throws -> Data {
        let url = URL(string: baseURL + path)!
        let (data, _) = try await session.data(from: url)
        return data
    }

    public func post(path: String, body: Data) async throws -> Data {
        var request = URLRequest(url: URL(string: baseURL + path)!)
        request.httpMethod = "POST"
        request.httpBody = body
        let (data, _) = try await session.data(for: request)
        return data
    }
}

public protocol NetworkService {
    func fetch(url: URL) async throws -> Data
}

public struct Endpoint {
    public let path: String
    public let method: String
}
