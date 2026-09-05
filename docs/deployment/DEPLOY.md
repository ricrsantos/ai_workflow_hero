# DEPLOY.md — Build, Release, and Distribution

> Living document for the Hero CLI's build and release process. Edited in place as the process evolves; history of changes lives in `context/context-log.md` and git, not in this file. Source: grilling session decisions, 2026-07-20. **Hero 1.0:** breaking major from 0.9.x — see §3.1 and [ADR-018](../architecture/ADR-C01-001-hero-1-0.md#adr-018-breaking-major-upgrade-from-09x). **Hero 2.0.0:** breaking major for multi-harness — see §3.2 and [ADR-034](../architecture/ADR-C04-001-multi-harness.md#adr-034-hero-200-interactive-harness-install-tools-removed). **Hero 2.5.0:** opt-in Codex TUI harness — see §3.3 and [ADR-048](../architecture/ADR-C06-001-codex-adapter.md#adr-048-hero-250-opt-in-codex-minor).

## 1. Distribution Model

Hero is distributed as a single, self-contained Go binary (`hero`) via GitHub Releases on the project's public repository (BSD-2-Clause license). All assets (commands, skills, prompts, templates) are embedded into the binary via `embed.FS` — there is nothing to download separately. Optional official plugins may add matching platform artifacts; the Telegram plugin downloads its local daemon from the matching Hero GitHub Release via `hero plugin install telegram`.

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
- **Hero 1.0.0** is a **breaking major** relative to 0.9.x: SQLite becomes the sole Hero operational store; cycle markdown (`workflow.md` / `metrics.md`) ceases to be canonical; Runtime assets must call the CLI API for state transitions ([PRD-C01-001](../product/PRD-C01-001-hero-1-0.md), ADR-013/014/018).
- **Hero 2.0.0** is a **breaking major** relative to 1.x: `hero install --tools` is removed; agents require `harness` in `workflow-config.yml`; OpenCode is an opt-in TUI harness ([PRD-C04-001](../product/PRD-C04-001-multi-harness.md), ADR-034).
- **Hero 2.5.0** is a **minor** relative to 2.4.x: Codex is an additional opt-in TUI harness (`CodexAdapter`, `.codex/` projection). Cursor and OpenCode stay intact; upgrade does not auto-enable Codex ([PRD-C06-001](../product/PRD-C06-001-codex-adapter.md), ADR-048).

### 3.1 Upgrade from 0.9.x (1.0)

- `hero upgrade` must create/migrate the SQLite store under `.workflow-hero/`, refresh embedded Runtime assets, and document any one-time migration of in-flight cycles.
- Soft dual-mode (running forever on 0.9.x markdown cycles) is **out of scope** for 1.0.
- Platforms remain Linux/macOS amd64/arm64; release script and checksum flow unchanged (§4).

### 3.2 Upgrade from 1.x (2.0.0)

- `hero upgrade` marks **Cursor enabled**; does **not** auto-enable OpenCode or write `.opencode/` until `/hero-harness` or a new install selects it.
- Passing `--tools` is an error (no deprecation). Direct users to `hero install` / `/hero-harness`.
- SQLite schema migrates as needed for OpenCode serve registry (project `hero.db`).
- Platforms remain Linux/macOS amd64/arm64; release script and checksum flow unchanged (§4).

### 3.3 Upgrade to 2.5.0 (Codex)

- `hero upgrade` from 2.4.x does **not** auto-enable Codex or write `.codex/` until `/hero-harness` or a new install selects it.
- Cursor and OpenCode enabled flags are unchanged.
- SQLite schema migrates as needed for Codex app-server process registry (project `hero.db`).
- Platforms remain Linux/macOS amd64/arm64; release script and checksum flow unchanged (§4).

The version is not hardcoded in source; it is injected at build time from the current git tag via linker flags:

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
4. Run `hero install` inside the target project and select at least one harness (Cursor, OpenCode, and/or Codex). `--tools` is not supported in 2.0.

`hero install` performs these deterministic checks before writing any files:

- Confirms the target directory is a git repository; if not, interactively offers to run `git init` (declining aborts installation — see [ADR-004](../architecture/ADR.md#adr-004-git-as-a-mandatory-prerequisite)).
- Copies commands, agents, skills, and templates from the embedded assets into harness projections (`.cursor/`, `.opencode/`, and/or `.codex/`) and `.workflow-hero/`.

## 7. Update & Removal

- `hero upgrade`: re-copies updated assets from the new binary version, but never silently overwrites a file the user has customized (detected via checksum comparison against what was originally installed); it warns instead, listing outdated files for manual merge.
- `hero uninstall`: removes only Hero-owned paths (`.cursor/agents/`, `.cursor/commands/hero-*.md`, `.cursor/skills/workflow-hero/`, `.cursor/skills/grilling/`, `.workflow-hero/`), preserving project artifacts (`AGENTS.md`, `context/`, `docs/`, `openspec/`).
- `hero doctor`: verifies installation integrity — presence of expected files/folders, version consistency between `hero.json` and the running binary, config file syntax, and git repository presence. Also warn-only soft secrets hygiene: missing `.env.example`, `.gitignore` not ignoring `.env`, or sensitive files tracked by git.
- Telegram plugin install/upgrade verifies a matching daemon artifact, preserves OS-vault credentials, migrates `.workflow-hero/tui.log` to `.workflow-hero/logs/tui.log` safely, and ensures Hero's managed `.gitignore` block ignores `.workflow-hero/logs/` without altering user entries.

## 8. Pricing Data Updates

`hero update-models` fetches a pre-structured pricing data file (JSON/YAML) published in the official Hero GitHub repository — maintained manually by Hero's maintainers whenever Cursor, OpenCode, or Codex pricing changes — and rewrites the local `models/*.yml` files. It never scrapes or parses HTML pricing pages. Catalog keys include Cursor slugs, OpenCode-native ids (`provider/model`), and Codex-native ids so TUI metrics resolve ([PRD-C04-001 §4.10](../product/PRD-C04-001-multi-harness.md#410-pricing-catalog--mandatory-implementation-task), [PRD-C06-001 §4.8](../product/PRD-C06-001-codex-adapter.md#48-hero-model-and-catalog)). Do not invent USD rates for ChatGPT-subsidized Codex models; unknown ids stay cost-zero with a warning.

## 9. References

- Platform/versioning/release rationale: [ADR-001](../architecture/ADR.md#adr-001-go-cobra-and-embedfs-for-cli-distribution), [ADR-010](../architecture/ADR.md#adr-010-manual-release-process-via-a-single-script)
- CLI vs. Runtime boundary (why `hero sync` does not exist as a CLI command): [ADR-003](../architecture/ADR.md#adr-003-cli-vs-runtime-separation)
- Full design discussion: [docs/idea/ai_workflow_hero.md](../idea/ai_workflow_hero.md)
