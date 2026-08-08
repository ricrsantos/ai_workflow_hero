// Package harness defines the HarnessAdapter interface and execution types used
// to push stage work into an IDE/harness (Cursor in V1). Full contract: IsAvailable,
// sessions, Execute, Cancel, Status, and Dispatch (ADR-025 / design D2).
//
// C2 also exports marker detection (DetectMarkers / KnownMarkers) for
// warn-only install/doctor checks (ADR-022).
package harness
