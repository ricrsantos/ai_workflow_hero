// Package cycle provides CLI-as-API Cobra commands and service façades over
// the Hero store and engine (status, metrics, events, approve, cycle lifecycle).
//
// Archive orchestration (ADR-023): when Planning knows the OpenSpec change slug,
// persist it with hero cycle openspec-change <slug> before archive. Archive
// resolves the name (stored → 0/1/N heuristic under openspec/changes/), runs
// openspec archive <name> -y when that change dir is still active, then moves
// the Hero cycle folder. Active entries under docs/idea are moved to
// docs/idea/archive as part of that filesystem archive; docs/idea/README.md and
// docs/idea/tobe are preserved. If the linked change is already archived on
// disk, the OpenSpec CLI step is skipped so a missing sandbox PATH cannot block
// Hero archive. The default runner also searches nvm/fnm/volta/user bin dirs
// when PATH is stripped.
package cycle
