package main

type Base struct{ N int }

type Derived struct {
    Base
    Extra string
}
