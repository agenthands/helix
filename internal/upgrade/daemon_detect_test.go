package upgrade

import "testing"

func TestRunningInDaemonTrue(t *testing.T) {
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "1")
	if !RunningInDaemon() {
		t.Fatalf("RunningInDaemon() = false, want true with HELIX_RUNNING_AS_DAEMON=1")
	}
}

func TestRunningInDaemonFalseUnset(t *testing.T) {
	// t.Setenv with empty value still sets the var to ""; we want it absent.
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "")
	if RunningInDaemon() {
		t.Fatalf("RunningInDaemon() = true, want false with empty HELIX_RUNNING_AS_DAEMON")
	}
}

func TestRunningInDaemonFalseWrongValue(t *testing.T) {
	// Only "1" counts. Other values (truthy in a shell sense or not) are false.
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "true")
	if RunningInDaemon() {
		t.Fatalf("RunningInDaemon() = true, want false with HELIX_RUNNING_AS_DAEMON=true")
	}
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "0")
	if RunningInDaemon() {
		t.Fatalf("RunningInDaemon() = true, want false with HELIX_RUNNING_AS_DAEMON=0")
	}
}
