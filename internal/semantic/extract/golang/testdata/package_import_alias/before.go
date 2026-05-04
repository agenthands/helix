package main

import (
    f "fmt"
    s "strings"
)

func Up(x string) string {
    f.Println(x)
    return s.ToUpper(x)
}
