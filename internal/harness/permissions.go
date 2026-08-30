package harness

import "strings"

// PermissionProfile controls how a harness handles approval requests for one
// Hero project. Profiles are deliberately small and portable; each adapter
// maps them to the permissions its native harness supports.
type PermissionProfile string

const (
	// PermissionProfileAsk requires a user decision for every native approval.
	PermissionProfileAsk PermissionProfile = "ask"
	// PermissionProfileAutoProject pre-approves only operations the adapter can
	// confine to the project workspace. Network, MCP, shell, and external-path
	// access remain subject to the harness's approval flow.
	PermissionProfileAutoProject PermissionProfile = "auto-project"
	// PermissionProfileAutoAll enables the native harness's unrestricted mode.
	// It can approve shell, network, MCP, and external-path operations.
	PermissionProfileAutoAll PermissionProfile = "auto-all"
)

// NormalizePermissionProfile returns the conservative profile for blank or
// unknown persisted values, keeping old hero.json files safe by default.
func NormalizePermissionProfile(profile PermissionProfile) PermissionProfile {
	switch PermissionProfile(strings.TrimSpace(strings.ToLower(string(profile)))) {
	case PermissionProfileAutoProject, PermissionProfileAutoAll:
		return PermissionProfile(strings.TrimSpace(strings.ToLower(string(profile))))
	default:
		return PermissionProfileAsk
	}
}

// PermissionProfileLabel returns concise user-facing profile copy.
func PermissionProfileLabel(profile PermissionProfile) string {
	switch NormalizePermissionProfile(profile) {
	case PermissionProfileAutoProject:
		return "Auto approve in project"
	case PermissionProfileAutoAll:
		return "Auto approve every time (Yolo)"
	default:
		return "Ask every time"
	}
}
