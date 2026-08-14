package cycle

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const maxDiscoveredArtifacts = 500

// ArtifactsView wraps artifact listing.
type ArtifactsView struct {
	CycleNumber int              `json:"cycleNumber"`
	Artifacts   []store.Artifact `json:"artifacts"`
}

// Artifacts returns store metadata plus files generated for the active cycle.
func (s *Service) Artifacts() (ArtifactsView, error) {
	if s == nil || s.Store == nil {
		return ArtifactsView{}, nil
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		if errors.Is(err, store.ErrNoActiveCycle) {
			return ArtifactsView{Artifacts: nil}, nil
		}
		return ArtifactsView{}, err
	}
	stored, err := s.Store.ListArtifacts(c.ID)
	if err != nil {
		return ArtifactsView{}, err
	}
	discovered := s.discoverCycleArtifacts(c)
	return ArtifactsView{
		CycleNumber: c.Number,
		Artifacts:   mergeArtifacts(stored, discovered),
	}, nil
}

func mergeArtifacts(stored, discovered []store.Artifact) []store.Artifact {
	seen := make(map[string]struct{}, len(stored)+len(discovered))
	out := make([]store.Artifact, 0, len(stored)+len(discovered))
	for _, a := range stored {
		key := normalizeArtifactPath(a.Path)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		a.Path = key
		out = append(out, a)
	}
	for _, a := range discovered {
		key := normalizeArtifactPath(a.Path)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		a.Path = key
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt < out[j].CreatedAt
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func (s *Service) discoverCycleArtifacts(c store.Cycle) []store.Artifact {
	if s.ProjectDir == "" {
		return nil
	}
	var out []store.Artifact
	seen := make(map[string]struct{})
	add := func(a store.Artifact) {
		if len(out) >= maxDiscoveredArtifacts {
			return
		}
		key := normalizeArtifactPath(a.Path)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if a.Kind == "" {
			a.Kind = artifactKind(key)
		}
		if a.Label == "" {
			a.Label = artifactLabel(key)
		}
		a.Path = key
		out = append(out, a)
	}

	s.walkArtifactDir(cursoradapter.HeroCurrentCycleDir, add)
	if slug := strings.TrimSpace(c.OpenspecChange); slug != "" && !strings.Contains(slug, "..") {
		s.walkArtifactDir(filepath.Join("openspec", "changes", slug), add)
	}
	for _, a := range s.artifactsFromDocumentsJSON(c.Number) {
		add(a)
	}
	for _, dir := range []string{
		filepath.Join("docs", "product"),
		filepath.Join("docs", "architecture"),
		filepath.Join("docs", "testing"),
		filepath.Join("docs", "deployment"),
		filepath.Join("docs", "ui"),
	} {
		s.walkArtifactDirMatchingCycle(dir, c.Number, add)
	}
	return out
}

func (s *Service) walkArtifactDir(relDir string, add func(store.Artifact)) {
	abs := filepath.Join(s.ProjectDir, relDir)
	_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "archive" {
				return fs.SkipDir
			}
			if strings.HasPrefix(name, ".") && path != abs {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipArtifactFile(name) {
			return nil
		}
		rel, err := filepath.Rel(s.ProjectDir, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		created := ""
		if err == nil {
			created = info.ModTime().UTC().Format(time.RFC3339)
		}
		add(store.Artifact{Path: rel, CreatedAt: created})
		return nil
	})
}

func (s *Service) walkArtifactDirMatchingCycle(relDir string, cycleNumber int, add func(store.Artifact)) {
	abs := filepath.Join(s.ProjectDir, relDir)
	_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != abs {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipArtifactFile(d.Name()) {
			return nil
		}
		if !filenameMatchesCycle(d.Name(), cycleNumber) {
			return nil
		}
		rel, err := filepath.Rel(s.ProjectDir, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		created := ""
		if err == nil {
			created = info.ModTime().UTC().Format(time.RFC3339)
		}
		add(store.Artifact{Path: rel, CreatedAt: created})
		return nil
	})
}

type documentsRegistry struct {
	Documents []documentsRegistryEntry `json:"documents"`
}

type documentsRegistryEntry struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Cycle    string `json:"cycle"`
	Purpose  string `json:"purpose"`
}

func (s *Service) artifactsFromDocumentsJSON(cycleNumber int) []store.Artifact {
	raw, err := os.ReadFile(filepath.Join(s.ProjectDir, cursoradapter.DocumentsJSONPath))
	if err != nil {
		return nil
	}
	var reg documentsRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil
	}
	var out []store.Artifact
	for _, doc := range reg.Documents {
		if !documentMatchesCycle(doc.Cycle, cycleNumber) {
			continue
		}
		rel := normalizeArtifactPath(doc.Path)
		if rel == "" {
			continue
		}
		abs := filepath.Join(s.ProjectDir, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			continue
		}
		label := firstNonEmpty(doc.Title, doc.Name, doc.Label, artifactLabel(rel))
		kind := firstNonEmpty(doc.Kind, strings.ToLower(doc.Category), artifactKind(rel))
		out = append(out, store.Artifact{
			Path:      rel,
			Kind:      kind,
			Label:     label,
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return out
}

func documentMatchesCycle(cycleField string, cycleNumber int) bool {
	cycleField = strings.TrimSpace(cycleField)
	if cycleField == "" || cycleNumber <= 0 {
		return false
	}
	s := strings.TrimPrefix(strings.ToUpper(cycleField), "C")
	n, err := strconv.Atoi(s)
	return err == nil && n == cycleNumber
}

func filenameMatchesCycle(name string, cycleNumber int) bool {
	if cycleNumber <= 0 {
		return false
	}
	pat := regexp.MustCompile(`(?i)(?:^|[-_./])C0*` + regexp.QuoteMeta(strconv.Itoa(cycleNumber)) + `(?:[-_.]|$)`)
	return pat.MatchString(name)
}

func shouldSkipArtifactFile(name string) bool {
	if name == "" || name == ".lock" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

func normalizeArtifactPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	if strings.Contains(p, "..") {
		return ""
	}
	return p
}

func artifactKind(rel string) string {
	rel = strings.ToLower(rel)
	base := strings.ToLower(filepath.Base(rel))
	switch {
	case strings.Contains(rel, "workflow-config"):
		return "config"
	case strings.Contains(rel, "browser-ui"):
		return "browser-ui"
	case strings.Contains(rel, "openspec"):
		return "spec"
	case strings.HasPrefix(base, "prd") || strings.Contains(rel, "/prd"):
		return "prd"
	case strings.HasPrefix(base, "adr") || strings.Contains(rel, "/adr"):
		return "adr"
	case strings.HasPrefix(base, "ui-") || strings.Contains(rel, "/ui-"):
		return "ui"
	case strings.Contains(rel, "testing"):
		return "testing"
	case strings.Contains(rel, "deploy"):
		return "deploy"
	case strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml"):
		return "config"
	default:
		return "file"
	}
}

func artifactLabel(rel string) string {
	base := filepath.Base(rel)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		return rel
	}
	return base
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
