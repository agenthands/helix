pub trait Animal {
    fn speak(&self);
}

pub struct Dog;

impl Animal for Dog {
    fn speak(&self) {}
}
