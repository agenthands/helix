mod greeter;
use greeter::Greeter;

fn helper() -> String {
    "hello".to_string()
}

struct DemoStruct {
    value: i32,
}

impl DemoStruct {
    fn get_value(&self) -> i32 {
        self.value
    }
}

fn unused_func() {}

fn using_helper() -> String {
    helper()
}

fn main() {
    let g = greeter::SimpleGreeter;
    println!("{}", g.greet("World"));
    println!("{}", helper());
}
