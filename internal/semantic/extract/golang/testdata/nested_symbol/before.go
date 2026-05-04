package main

func Outer() func() int {
    return func() int { return 42 }
}
