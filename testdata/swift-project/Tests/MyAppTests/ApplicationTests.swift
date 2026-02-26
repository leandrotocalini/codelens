import XCTest
@testable import MyApp

final class ApplicationTests: XCTestCase {
    func testAppCreation() {
        let config = AppConfig(environment: "test")
        let app = createApp(config: config)
        XCTAssertEqual(app.name, "MyApp")
    }

    func testEnvironment() {
        let env = Environment.development
        XCTAssertEqual(env.rawValue, "development")
    }
}
