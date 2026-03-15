package debug

import (
	"os"
	"sync"
	"testing"
)

func resetState() {
	once = sync.Once{}
	logger = nil
}

func TestLogNoOpsWhenDisabled(t *testing.T) {
	os.Unsetenv("ZMUX_DEBUG")
	enabled = false
	resetState()

	// Should not panic
	Log("test message", "key", "value")
	Error("test error", "key", "value")
}

func TestEnabledReturnsFalseByDefault(t *testing.T) {
	os.Unsetenv("ZMUX_DEBUG")
	enabled = false
	if Enabled() {
		t.Error("expected Enabled() to return false when ZMUX_DEBUG is not set")
	}
}

func TestEnabledReturnsTrueWhenSet(t *testing.T) {
	enabled = true
	if !Enabled() {
		t.Error("expected Enabled() to return true")
	}
	enabled = false
}
