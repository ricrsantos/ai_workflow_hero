# DEPLOY.md — Build, Release, and Distribution

> Living document for the Hero CLI's build and release process. Edited in place as the process evolves; history of changes lives in `context/context-log.md` and git, not in this file. Source: grilling session decisions, 2026-07-20.

## 1. Distribution Model

Hero is distributed as a single, self-contained Go binary (`hero`) via GitHub Releases on the project's public repository (BSD-2-Clause license). All assets (commands, skills, prompts, templates) are embedded into the binary via `embed.FS` — there is nothing to download separately.

## 2. Target Platforms (V1)

| OS | Architecture |
|---|---|
| Linux | `amd64` |
| Linux | `arm64` |
| macOS (Darwin) | `amd64` |
| macOS (Darwin) | `arm64` |

Windows is **out of scope for V1** (see [PRD.md §2.3](../product/PRD.md#23-v2-scope-out-of-scope-for-v1)) and is planned for V2.

## 3. Versioning

- Hero follows **Semantic Versioning** (`vMAJOR.MINOR.PATCH`, e.g. `v1.2.0`).
- The version is not hardcoded in source; it is injected at build time from the current git tag via linker flags:

```bash
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0)" -o hero ./cmd/hero
```

- `assets.version` (recorded in the installed project's `.workflow-hero/config/hero.json`) is **always equal to** `cli.version`, since assets are embedded in the same binary and cannot diverge from it (see [ADR-001](../architecture/ADR.md#adr-001-go-cobra-and-embedfs-for-cli-distribution)).

## 4. Build & Release Process (V1: manual, script-assisted)

V1 does not use CI/CD. Releases are cut manually by the maintainer, but the repetitive cross-compilation work is automated by a single script.

**Testing gate**: `go test ./...` must pass with zero failures before tagging a release commit. If any test fails, analyze and fix it, then re-run the suite — never tag or run `scripts/release.sh` against a repository in a failing state (see [AGENTS.md — Testing](../../AGENTS.md#testing) and [ADR-009](../architecture/ADR.md#adr-009-test-real-dependencies-over-mocks)).

### 4.1 `scripts/release.sh`

Responsibilities of the script, run from a clean, tagged commit:

1. Read the current version from `git describe --tags --abbrev=0`.
2. Cross-compile the 4 target combinations listed in §2, using `GOOS`/`GOARCH` and the `-ldflags` version injection from §3.
3. Name each binary `hero_<version>_<os>_<arch>` (e.g. `hero_v1.2.0_linux_amd64`).
4. Generate a `checksums.txt` file containing the SHA256 checksum of every binary produced.
5. Place all output artifacts in a local `dist/` directory (gitignored).

Usage:

```bash
./scripts/release.sh
# → dist/hero_v1.2.0_linux_amd64
# → dist/hero_v1.2.0_linux_arm64
# → dist/hero_v1.2.0_darwin_amd64
# → dist/hero_v1.2.0_darwin_arm64
# → dist/checksums.txt
```

### 4.2 Publishing

1. Run `go test ./...` and confirm it passes (see Testing gate above).
2. Tag the release commit: `git tag v1.2.0 && git push origin v1.2.0`.
3. Run `./scripts/release.sh`.
4. Manually create a GitHub Release for the tag and upload all files from `dist/` (4 binaries + `checksums.txt`).
5. Write release notes summarizing changes since the previous tag.

> V2 candidate: automate steps 2–3 with GoReleaser + GitHub Actions once release frequency justifies the investment (see [PRD.md §2.3](../product/PRD.md#23-v2-scope-out-of-scope-for-v1)).

## 5. Integrity Verification

- `checksums.txt` (SHA256) is published alongside every release's binaries.
- Verification is manual, performed by the user after download:

```bash
sha256sum -c checksums.txt --ignore-missing
```

- No GPG signing in V1. This can be added in a future version if demand emerges.

## 6. Installation (End-User Flow)

1. Download the binary matching the user's OS/architecture from the repository's GitHub Releases page.
2. (Optional but recommended) Verify its checksum against `checksums.txt`.
3. Place the binary in a directory on the system `PATH`.
4. Run `hero install --tools cursor` inside the target project.

`hero install` performs these deterministic checks before writing any files:

- Confirms the target directory is a git repository; if not, interactively offers to run `git init` (declining aborts installation — see [ADR-004](../architecture/ADR.md#adr-004-git-as-a-mandatory-prerequisite)).
- Copies commands, agents, skills, and templates from the embedded assets into `.cursor/` and `.workflow-hero/`.

## 7. Update & Removal

- `hero upgrade`: re-copies updated assets from the new binary version, but never silently overwrites a file the user has customized (detected via checksum comparison against what was originally installed); it warns instead, listing outdated files for manual merge.
- `hero uninstall`: removes only Hero-owned paths (`.cursor/agents/`, `.cursor/commands/hero-*.md`, `.cursor/skills/workflow-hero/`, `.cursor/skills/grilling/`, `.workflow-hero/`), preserving project artifacts (`AGENTS.md`, `context/`, `docs/`, `openspec/`).
- `hero doctor`: verifies installation integrity — presence of expected files/folders, version consistency between `hero.json` and the running binary, config file syntax, and git repository presence. Also warn-only soft secrets hygiene: missing `.env.example`, `.gitignore` not ignoring `.env`, or sensitive files tracked by git.

## 8. Pricing Data Updates

`hero update-models` fetches a pre-structured pricing data file (JSON/YAML) published in the official Hero GitHub repository — maintained manually by Hero's maintainers whenever Cursor's pricing changes — and rewrites the local `models/*.yml` files. It never scrapes or parses HTML pricing pages.

## 9. References

- Platform/versioning/release rationale: [ADR-001](../architecture/ADR.md#adr-001-go-cobra-and-embedfs-for-cli-distribution), [ADR-010](../architecture/ADR.md#adr-010-manual-release-process-via-a-single-script)
- CLI vs. Runtime boundary (why `hero sync` does not exist as a CLI command): [ADR-003](../architecture/ADR.md#adr-003-cli-vs-runtime-separation)
- Full design discussion: [docs/idea/ai_workflow_hero.md](../idea/ai_workflow_hero.md)
