package harness

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func detectMarkers(projectRoot string, configuredTools []string, exists func(string) bool) (DetectionResult, error) {
	configured := map[string]bool{}
	for _, t := range configuredTools {
		if t = strings.TrimSpace(strings.ToLower(t)); t != "" {
			configured[t] = true
		}
	}

	var res DetectionResult
	knownTools := map[string]MarkerDir{}
	for _, m := range KnownMarkers {
		knownTools[m.ToolID] = m
		if exists(filepath.Join(projectRoot, m.Dir)) {
			res.Present = append(res.Present, m)
			// Claude has a native adapter, but a marker is only managed when
			// Hero explicitly configured the harness. Keeping an unconfigured
			// Claude marker in UnsupportedPresent preserves the existing
			// warn-only API used by install/doctor.
			if !m.Supported || (m.ToolID == "claude" && !configured[m.ToolID]) {
				res.UnsupportedPresent = append(res.UnsupportedPresent, m)
			}
		}
	}

	configuredNames := make([]string, 0, len(configured))
	for tool := range configured {
		configuredNames = append(configuredNames, tool)
	}
	sort.Strings(configuredNames)
	for _, tool := range configuredNames {
		m, ok := knownTools[tool]
		if !ok {
			res.ExtraConfigured = append(res.ExtraConfigured, tool)
			continue
		}
		if m.Supported && !exists(filepath.Join(projectRoot, m.Dir)) {
			res.MissingConfigured = append(res.MissingConfigured, tool)
		}
	}
	sort.SliceStable(res.Present, func(i, j int) bool { return res.Present[i].Dir < res.Present[j].Dir })
	sort.SliceStable(res.UnsupportedPresent, func(i, j int) bool {
		return res.UnsupportedPresent[i].Dir < res.UnsupportedPresent[j].Dir
	})
	sort.Strings(res.MissingConfigured)
	sort.Strings(res.ExtraConfigured)
	return res, nil
}
