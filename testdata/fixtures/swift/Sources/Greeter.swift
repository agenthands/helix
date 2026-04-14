protocol Greetable {
    func greet(name: String) -> String
}

class Greeter: Greetable {
    func greet(name: String) -> String {
        return "Hello, \(name)!"
    }
}
