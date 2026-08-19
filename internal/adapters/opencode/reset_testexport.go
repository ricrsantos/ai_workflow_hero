package opencode

import "time"

// SetServeResetDelayForTest overrides the pause between StopServe and ensureServe.
func SetServeResetDelayForTest(d time.Duration) {
	serveResetDelay = d
}

// ServeResetDelayForTest returns the current reset delay (for test cleanup).
func ServeResetDelayForTest() time.Duration {
	return serveResetDelay
}
