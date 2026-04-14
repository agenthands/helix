func helper() -> String {
    return "hello"
}

struct DemoStruct {
    var value: Int

    func getValue() -> Int {
        return value
    }
}

func unusedFunc() {}

func usingHelper() -> String {
    return helper()
}

let g = Greeter()
print(g.greet(name: "World"))
print(helper())
