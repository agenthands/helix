// Phase 63 P63-02 Task 2: helpers for cas_property_test (kept in a
// separate file so the platform shims stay easy to grep).

package compact_test

import "os"

func getwdPlatform() (string, error) { return os.Getwd() }
func chdirPlatform(d string) error   { return os.Chdir(d) }
