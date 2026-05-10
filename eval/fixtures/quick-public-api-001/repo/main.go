package main

import "fmt"

// Greeter greets a user.
type Greeter struct {
	Name string
}

// Hello returns a greeting string.
func (g *Greeter) Hello() (string, error) {
	return fmt.Sprintf("Hello, %s!", g.Name), nil
}

func main() {
	g := &Greeter{Name: "World"}
	msg, err := g.Hello()
	if err != nil {
		panic(err)
	}
	fmt.Println(msg)
}
