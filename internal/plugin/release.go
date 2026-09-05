package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
)

// DefaultReleaseRepo is the GitHub repository that publishes Hero releases.
const DefaultReleaseRepo = "ricrsantos/ai_workflow_hero"

// DownloadOptions configures fetching the platform-matched daemon from GitHub
// Releases (telegram-plugin R1; ADR-059).
type DownloadOptions struct {
	Version    string
	Repo       string
	GOOS       string
	GOARCH     string
	HTTPClient *http.Client
	// ReleaseBaseURL overrides the default GitHub Releases download root for tests.
	// When set, the URL is {ReleaseBaseURL}/{tag}/{artifactName}.
	ReleaseBaseURL string
}

// DaemonArtifactFileName returns the release asset name for the daemon binary
// (e.g. hero-telegram-daemon_v3.0.0_linux_amd64).
func DaemonArtifactFileName(version, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s", telegram.DaemonBinaryName, releaseTag(version), goos, goarch)
}

// DaemonReleaseURL builds the GitHub Releases download URL for the daemon artifact.
func DaemonReleaseURL(repo, version, goos, goarch string) string {
	return daemonDownloadURL("", repo, version, goos, goarch)
}

func daemonDownloadURL(baseURL, repo, version, goos, goarch string) string {
	tag := releaseTag(version)
	name := DaemonArtifactFileName(version, goos, goarch)
	if baseURL != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), tag, name)
	}
	if repo == "" {
		repo = DefaultReleaseRepo
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, name)
}

// DownloadDaemonArtifact fetches the platform-matched daemon binary from GitHub
// Releases into a temporary executable file. The caller must invoke cleanup when
// done.
func DownloadDaemonArtifact(ctx context.Context, opts DownloadOptions) (path string, cleanup func(), err error) {
	if opts.Version == "" {
		return "", nil, fmt.Errorf("hero version is required to download the telegram daemon")
	}
	if opts.Repo == "" {
		opts.Repo = DefaultReleaseRepo
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if !isSupportedPlatform(opts.GOOS, opts.GOARCH) {
		return "", nil, fmt.Errorf(
			"telegram plugin is not published for %s/%s (supported: linux/darwin amd64/arm64)",
			opts.GOOS, opts.GOARCH,
		)
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	url := daemonDownloadURL(opts.ReleaseBaseURL, opts.Repo, opts.Version, opts.GOOS, opts.GOARCH)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build download request: %w", err)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf(
			"download %s: HTTP %d — verify Hero %s is released on GitHub with a %s/%s daemon artifact",
			url, resp.StatusCode, releaseTag(opts.Version), opts.GOOS, opts.GOARCH,
		)
	}

	tmp, err := os.CreateTemp("", telegram.DaemonBinaryName+"-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary daemon file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanupFn := func() { _ = os.Remove(tmpPath) }

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		cleanupFn()
		return "", nil, fmt.Errorf("save daemon download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("close daemon download: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("chmod daemon download: %w", err)
	}

	fi, err := os.Stat(tmpPath)
	if err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("stat daemon download: %w", err)
	}
	if fi.Size() == 0 {
		cleanupFn()
		return "", nil, fmt.Errorf("downloaded daemon artifact is empty")
	}

	return tmpPath, cleanupFn, nil
}

// InstallTelegramFromRelease downloads the matching daemon from GitHub Releases
// and installs it under ~/.workflow-hero/plugins/telegram/.
func InstallTelegramFromRelease(ctx context.Context, version string, now time.Time) (Manifest, error) {
	pluginDir, err := telegram.PluginDir(telegram.PluginName)
	if err != nil {
		return Manifest{}, err
	}
	src, cleanup, err := DownloadDaemonArtifact(ctx, DownloadOptions{Version: version})
	if err != nil {
		return Manifest{}, err
	}
	defer cleanup()
	return InstallTelegram(pluginDir, src, version, now)
}

func releaseTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func isSupportedPlatform(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin":
		return goarch == "amd64" || goarch == "arm64"
	default:
		return false
	}
}
