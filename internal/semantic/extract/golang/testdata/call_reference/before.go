package main

func Inner() int { return 1 }

func Outer() int { return Inner() }
