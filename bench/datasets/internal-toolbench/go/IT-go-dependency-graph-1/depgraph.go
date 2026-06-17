// Package depgraph is the root of the internal-toolbench/go dependency_graph
// fixture (IT-go-dependency-graph-1). The actual assertion lives in
// wiring_test.go, which imports every core-dependent package (alpha, beta,
// gamma) and checks each surfaces core.Tag().
package depgraph
