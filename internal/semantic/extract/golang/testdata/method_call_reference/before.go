package main

type S struct{}

func (s S) Do() {}

func Run(s S) { s.Do() }
