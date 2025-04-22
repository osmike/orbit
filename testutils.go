package orbit

import (
	"testing"
	"time"
)

// waitForCondition polls the condition function until it returns true or timeout is reached.
// It fails the test with a fatal error if the timeout is reached.
func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for condition")
}

// toInt safely converts various numeric interface{} types into int.
// It supports int and float64; all other types return 0.
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	default:
		return 0
	}
}
