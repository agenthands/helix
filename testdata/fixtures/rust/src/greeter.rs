pub trait Greeter {
    fn greet(&self, name: &str) -> String;
}

pub struct SimpleGreeter;

impl Greeter for SimpleGreeter {
    fn greet(&self, name: &str) -> String {
        format!("Hello, {}!", name)
    }
}
