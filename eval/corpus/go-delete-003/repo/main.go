package main

import (
	"fmt"
)

// DeprecatedFormat is a deprecated format method on type Item.
type Item struct{ name string }

func (i Item) DeprecatedFormat() string {
	return "old:" + i.name
}

func (i Item) NewFormat() string {
	return "new:" + i.name
}

// FormatItem calls DeprecatedFormat.
func FormatItem(i Item) string {
	return i.DeprecatedFormat()
}

func main() {
	result := FormatItem(Item{name: "x"})
	fmt.Println(result)
}
