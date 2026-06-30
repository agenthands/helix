pub struct Person {
    name: String,
    age: u32,
}

impl Person {
    pub fn name(&self) -> &str {
        &self.name
    }
}
