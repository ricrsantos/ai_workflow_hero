package harness

import (
	"os"
	"path/filepath"
)

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func detectMarkers(projectRoot string, configuredTools []string, exists func(string) bool) (DetectionResult, error) {
	configured := map[string]bool{}
	for _, t := range configuredTools {
		if t != "" {
			configured[t] = true
		}
	}

	var res DetectionResult
	knownTools := map[string]MarkerDir{}
	for _, m := range KnownMarkers {
		knownTools[m.ToolID] = m
		if exists(filepath.Join(projectRoot, m.Dir)) {
			res.Present = append(res.Present, m)
			if !m.Supported {
				res.UnsupportedPresent = append(res.UnsupportedPresent, m)
			}
		}
	}

	for tool := range configured {
		m, ok := knownTools[tool]
		if !ok {
			res.ExtraConfigured = append(res.ExtraConfigured, tool)
			continue
		}
		if m.Supported && !exists(filepath.Join(projectRoot, m.Dir)) {
			res.MissingConfigured = append(res.MissingConfigured, tool)
		}
	}
	return res, nil
}
