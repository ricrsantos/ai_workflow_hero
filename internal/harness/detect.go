package harness

// MarkerDir describes a known IDE/harness filesystem marker at the project root.
// Used by install/doctor warn-only detection (ADR-022; design D4).
type MarkerDir struct {
	// Dir is the project-relative directory name (e.g. ".cursor").
	Dir string
	// ToolID is the cli.tools identifier (e.g. "cursor").
	ToolID string
	// Supported is true when Hero ships assets/adapters for this harness in this version.
	Supported bool
}

// KnownMarkers is the versioned table of harness directories Hero can detect.
var KnownMarkers = []MarkerDir{
	{Dir: ".cursor", ToolID: "cursor", Supported: true},
	{Dir: ".opencode", ToolID: "opencode", Supported: true},
	{Dir: ".codex", ToolID: "codex", Supported: true},
	{Dir: ".claude", ToolID: "claude", Supported: false},
	{Dir: ".windsurf", ToolID: "windsurf", Supported: false},
}

// DetectionResult is the outcome of comparing filesystem markers to cli.tools.
type DetectionResult struct {
	// Present lists markers whose directories exist under the project root.
	Present []MarkerDir
	// UnsupportedPresent are Present markers that are not Supported.
	UnsupportedPresent []MarkerDir
	// MissingConfigured are Supported tools listed in cli.tools whose marker dir is absent.
	MissingConfigured []string
	// ExtraConfigured are cli.tools entries with no known marker mapping.
	ExtraConfigured []string
}

// DetectMarkers scans projectRoot for known harness directories and compares them to
// configuredTools (typically hero.json → cli.tools). Detection is warn-only:
// callers must not install unsupported harness assets (ADR-022).
func DetectMarkers(projectRoot string, configuredTools []string) (DetectionResult, error) {
	return detectMarkers(projectRoot, configuredTools, dirExists)
}

// MarkerDetector is the shared detect API used by doctor and install.
// Prefer DetectMarkers for the default filesystem implementation.
type MarkerDetector func(projectRoot string, configuredTools []string) (DetectionResult, error)
