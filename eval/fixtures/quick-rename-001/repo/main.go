package main

type Foo struct{}

func (f *Foo) Hello() {}

func main() { _ = (&Foo{}).Hello }
