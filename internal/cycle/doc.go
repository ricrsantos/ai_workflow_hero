// Package cycle provides CLI-as-API Cobra commands and service façades over
// the Hero store and engine (status, metrics, events, approve, cycle lifecycle).
//
// Archive orchestration (ADR-023): when Planning knows the OpenSpec change slug,
// persist it with hero cycle openspec-change <slug> before archive. Archive
// resolves the name (stored → 0/1/N heuristic under openspec/changes/), runs
// openspec archive <name> -y, then moves the Hero cycle folder.
package cycle
