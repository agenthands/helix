// Package beta depends on core. Label is DELIBERATELY empty until the agent
// wires core.Tag() in (D-05: every core-dependent package must be updated).
package beta

import "toolbench/depgraph/core"

// Label must return "core:" + core.Tag().
func Label() string {
	_ = core.Tag
	return ""
}
