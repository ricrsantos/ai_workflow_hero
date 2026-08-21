package codex

import "time"

// SetAppServerResetDelayForTest overrides the pause between StopAppServer and ensureAppServer.
func SetAppServerResetDelayForTest(d time.Duration) {
	appServerResetDelay = d
}

// AppServerResetDelayForTest returns the current reset delay (for test cleanup).
func AppServerResetDelayForTest() time.Duration {
	return appServerResetDelay
}
