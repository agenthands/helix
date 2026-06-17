package rename

// Describe is a second-file reference to Foo. A correct rename must update this
// call site too (find_references surfaces it).
func Describe() string {
	return "this is a " + Foo()
}
