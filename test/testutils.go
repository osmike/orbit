package test

import (
	"testing"
	"time"
)

// WaitForCondition polls the condition function until it returns true or timeout is reached.
// It fails the test with a fatal error if the timeout is reached.
func WaitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for condition")
}

// ToInt safely converts various numeric interface{} types into int.
// It supports int and float64; all other types return 0.
func ToInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	default:
		return 0
	}
}
