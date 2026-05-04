package main

type R interface{ Read() }

type RW interface {
    R
    Write()
}
