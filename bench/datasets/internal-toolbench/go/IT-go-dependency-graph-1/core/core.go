// Package core is the shared dependency in the internal-toolbench/go
// dependency_graph fixture (IT-go-dependency-graph-1). Packages alpha, beta, and
// gamma all import core; get_repo_map / get_context surfaces these edges. The
// scripted agent must wire core.Tag() into EVERY dependent's Label.
package core

// Tag is the shared version stamp every dependent package must surface.
func Tag() string {
	return "v1"
}
