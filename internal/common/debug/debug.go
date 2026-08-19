package debug

// Enabled is true when Hero runs with --debug (set from cmd/hero PersistentPreRun).
var Enabled bool

// SetEnabled updates the global debug flag for the current process.
func SetEnabled(v bool) {
	Enabled = v
}
