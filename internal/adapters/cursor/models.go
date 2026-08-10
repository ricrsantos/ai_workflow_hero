package cursor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// modelLineRE matches "slug - Display Name" lines from `agent models`.
var modelLineRE = regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9._-]*)\s+-\s+.+$`)

// ListModels implements harness.ModelLister via `agent models` (fallback `--list-models`).
func (a *Adapter) ListModels(ctx context.Context) ([]string, error) {
	spec, err := ResolveAgentCLI(a.LookPath)
	if err != nil {
		return nil, err
	}
	runner := a.runner()
	dir := a.ProjectDir

	res, err := runner.Run(ctx, dir, spec.Path, spec.BuildArgs("models"))
	models := ParseModelsOutput(res.Stdout)
	if len(models) == 0 {
		res2, err2 := runner.Run(ctx, dir, spec.Path, spec.BuildArgs("--list-models"))
		models = ParseModelsOutput(res2.Stdout)
		if len(models) == 0 {
			if err != nil {
				return nil, fmt.Errorf("list models: %w", err)
			}
			if err2 != nil {
				return nil, fmt.Errorf("list models: %w", err2)
			}
			return nil, fmt.Errorf("list models: empty output")
		}
	}
	return models, nil
}

// ParseModelsOutput extracts model slugs from `agent models` / `--list-models` stdout.
func ParseModelsOutput(raw []byte) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "available models") {
			continue
		}
		slug := ""
		if m := modelLineRE.FindStringSubmatch(line); len(m) == 2 {
			slug = m[1]
		} else if isBareModelSlug(line) {
			slug = line
		}
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}
	return out
}

func isBareModelSlug(s string) bool {
	if strings.ContainsAny(s, " \t") {
		return false
	}
	return modelLineRE.MatchString(s + " - x")
}
