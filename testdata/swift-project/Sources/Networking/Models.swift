import Foundation

public struct Response<T: Decodable> {
    public let data: T
    public let statusCode: Int
}

public enum NetworkError: Error {
    case invalidURL
    case serverError(Int)
    case decodingError
}
