import Foundation
import Networking

public protocol AppDelegate {
    func applicationDidFinishLaunching()
}

public class Application {
    public let name: String
    private var isRunning: Bool = false

    public init(name: String) {
        self.name = name
    }

    public func start() {
        isRunning = true
        print("Starting \(name)")
    }

    public func stop() {
        isRunning = false
    }

    private func cleanup() {
        // internal cleanup
    }
}

public struct AppConfig {
    public let environment: String
    public let debug: Bool

    public init(environment: String, debug: Bool = false) {
        self.environment = environment
        self.debug = debug
    }
}

public enum Environment: String {
    case development
    case staging
    case production
}

public typealias CompletionHandler = (Result<Data, Error>) -> Void

public func createApp(config: AppConfig) -> Application {
    return Application(name: "MyApp")
}

func internalHelper() -> String {
    return "helper"
}
