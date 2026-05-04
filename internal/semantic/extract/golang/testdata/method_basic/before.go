package main

type Counter struct{ n int }

func (c *Counter) Increment() { c.n++ }
