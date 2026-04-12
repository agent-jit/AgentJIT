# Deterministic Trace Compiler — Design Document

**Date:** 2026-04-10
**Status:** Implemented (2026-04-12)

## Motivation

The current AJ compiler sends all JSONL logs to Claude for pattern detection, parameterization, and script generation. This works well but costs ~10,000+ tokens per compile cycle. Many patterns — especially sequences of Bash commands — are structurally simple and can be detected, parameterized, and compiled entirely in Go with zero LLM tokens.

The goal is a **deterministic trace compiler** that handles "obvious" patterns end-to-end, routing only ambiguous patterns to the LLM as a fallback. A TUI (`aj trace`) lets users explore the trace graph interactively.

## Architecture Overview

```
JSONL logs
    |
[Trace Graph Builder]     <- builds directed graph from tool-call sequences
    |
[Hot Path Detector]       <- finds frequent subgraphs above threshold
    |
[Structural Differ]       <- aligns instances, extracts parameters
    |
[Confidence Scorer]       <- decides deterministic vs. LLM fallback
    |
  +-------------+
  |  confident?  |
  +-- yes --> [Template Codegen]    <- emits .sh/.ps1 from Bash tool calls
  +-- no  --> [LLM Compiler]       <- existing Claude-based compiler
    |
[Skill Output]            <- same format: SKILL.md + metadata.json + scripts/
```

Both backends produce identical output (skill directory structure), so the rest of AJ (linking, tracking, stats) works unchanged. The pluggability is at the `CompilerBackend` interface level.

## Trace Graph Data Structures

### Node

```go
type Node struct {
    ToolName   string            // e.g. "Bash", "Read", "Edit"
    InputShape map[string]string // structural fingerprint
    ID         uint64            // hash of (ToolName, InputShape)
}
```

### Edge

```go
type Edge struct {
    From, To   uint64   // node IDs
    Weight     int      // number of times this transition observed
    SessionIDs []string // which sessions contributed
}
```

### TraceGraph

```go
type TraceGraph struct {
    Nodes map[uint64]*Node
    Edges map[uint64]map[uint64]*Edge // adjacency list
}
```

### InputShape Fingerprinting

1. Parse tool input JSON.
2. For `Bash` commands: tokenize the command string, replace tokens that look variable (paths, UUIDs, k8s resource names, IPs) with `{VAR}` placeholders using heuristics.
3. For other tools: use the JSON key structure as the fingerprint, replacing values with type tags (`{STRING}`, `{NUMBER}`).
4. Hash the normalized shape to produce the node ID.

Two tool calls with the same `(ToolName, InputShape)` map to the **same node**. This is how `kubectl get pods -n staging` and `kubectl get pods -n production` collapse into one node with a `{VAR}` parameter.

## Hot Path Detection

A **hot path** is a frequently-traversed subgraph — a sequence of nodes that appears across multiple sessions above the configured threshold (`min_pattern_frequency`, default 3).

### Algorithm: Weighted DFS with Minimum Support

```
FindHotPaths(graph, minFrequency, minLength=2, maxLength=20):
    hotPaths = []

    for each node N in graph:
        DFS(path=[N], sessionSets=[N.sessions]):
            tail = path[-1]
            for each edge (tail -> next) in graph:
                commonSessions = sessionSets ∩ edge.SessionIDs
                if len(commonSessions) >= minFrequency:
                    newPath = path + [next]
                    if len(newPath) >= minLength:
                        hotPaths.append(newPath, commonSessions)
                    if len(newPath) < maxLength:
                        DFS(newPath, commonSessions)

    return pruneSubPaths(hotPaths)
```

Key properties:

- **Session intersection** ensures every step in the path was taken in the same sessions — no Frankenstein patterns stitched from different workflows.
- **Pruning** keeps only maximal paths (e.g., if A->B->C->D is hot, don't also emit A->B->C unless it has higher frequency).
- **maxLength cap** (20) prevents runaway traces.
- **Complexity** is bounded because the graph is sparse and we prune early via the frequency threshold.

## Structural Diffing & Parameterization

Once we have a hot path observed in N sessions, we go back to the **raw events** from those sessions to extract concrete values and diff them.

### Process

For each CandidatePattern:

1. Load raw events from each contributing session.
2. Align events to the pattern's node sequence.
3. Diff tool inputs across instances, field by field.
4. For `Bash` commands, tokenize and compare token-by-token:
   - All instances have same literal at position → keep as literal.
   - Values differ → mark as parameter.
5. For non-Bash tools: JSON structural diff (tracked for SKILL.md docs, not codegen'd).

### Parameter Naming Heuristics

- Token follows `-n` / `--namespace` → `NAMESPACE`
- Token matches path pattern (`/`, `.`, extension) → `FILE_PATH`
- Token follows known flag patterns → named after the flag
- Fallback: `ARG_1`, `ARG_2`, etc.

## Confidence Scoring & Routing

The confidence scorer decides whether a pattern goes to the deterministic backend or the LLM.

| Factor               | High confidence          | Low confidence                         |
|----------------------|--------------------------|----------------------------------------|
| Tool types           | All Bash calls           | Mixed Read/Edit/Bash with complex logic |
| Parameter consistency | Same positions vary      | Tokens shift position across instances  |
| Diff cleanliness     | ≤3 parameters per step   | >5 parameters or entire command varies  |
| Step count           | 2-8 steps                | >12 steps                              |
| Exit codes           | All instances exit 0     | Mixed success/failure                  |

### Formula

```
score = 1.0
if hasMixedNonBashTools:     score -= 0.3
if avgParamsPerStep > 3:     score -= 0.2
if anyTokenPositionShifts:   score -= 0.3
if pathLength > 12:          score -= 0.1
if mixedExitCodes:           score -= 0.2

route = "deterministic" if score >= 0.6 else "llm"
```

The threshold (0.6) is configurable via `compile.deterministic_threshold`. Setting it to 1.0 forces everything to the LLM; 0.0 forces everything deterministic.

### TieredCompiler Orchestrator

```go
patterns := tracer.FindHotPaths(graph)
patterns  = tracer.Parameterize(patterns)

deterministicBatch, llmBatch := scorer.Route(patterns)
skills1 := deterministicBackend.Compile(deterministicBatch)
skills2 := llmBackend.Compile(llmBatch) // only if llmBatch non-empty
return merge(skills1, skills2)
```

## Template Codegen (Deterministic Backend)

Generates scripts from parameterized Bash patterns. No LLM involved.

### Script Generation

1. Emit shebang and `set -euo pipefail`.
2. Map parameters to positional args with usage validation.
3. Emit each Bash step with parameter substitutions.
4. Non-Bash steps are skipped (agent reasoning scaffolding).

### ROI Calculation (in Go)

```
stochasticCost  = avgInputTokens + avgOutputTokens  // char counts / 4
deterministicCost = 200  // skill invocation overhead
savingsPerCall  = stochasticCost - deterministicCost
totalSavings    = savingsPerCall * frequency
```

### SKILL.md Trigger Clause

Inferred from the first command in the pattern. E.g., `kubectl get pods -n $NAMESPACE` generates: `TRIGGER when: user asks to get pod status or list pods in a namespace`.

Skills generated by this backend are marked `"generated_by": "aj-deterministic"` in metadata.json, so a future LLM pass can polish descriptions if desired.

## TUI: `aj trace`

An interactive terminal UI (bubbletea) for exploring the trace graph.

### Hot Paths List View

```
+-- aj trace --------------------------------------------------------+
|                                                                     |
|  Hot Paths (sorted by frequency)              [Tab] switch          |
|  ------------------------------------------------------------------ |
|  > kubectl get pods -> kubectl logs -> kubectl describe       (12x) |
|    git status -> git diff -> git add -> git commit             (9x) |
|    az login -> az aks get-credentials -> kubectl ...           (7x) |
|    docker build -> docker tag -> docker push                   (5x) |
|                                                                     |
|  [Enter] expand  [/] filter  [c] compile  [q] quit                 |
+---------------------------------------------------------------------+
```

### Expanded Path Detail View

Shows frequency, confidence score, estimated savings, steps with variant counts, extracted parameters, and contributing sessions.

### Additional Views

- **`d` — Diff view:** Side-by-side structural diff of instances with parameters highlighted.
- **`g` — Graph view:** ASCII rendering of the subgraph with edge weights.
- **`c` — Compile:** Run the appropriate compiler on selected pattern, preview skill before writing.

### Commands

- `/` — filter by tool name, minimum frequency, or session date range
- `Tab` — switch between hot paths list, full graph overview, and session timeline
- `s` — sort by frequency, savings, confidence, or recency

## Package Layout

```
internal/
+-- trace/                    <- NEW: core trace algorithm
|   +-- graph.go              # TraceGraph, Node, Edge structs & builder
|   +-- hotpath.go            # FindHotPaths (DFS with session intersection)
|   +-- diff.go               # Structural differ & parameterization
|   +-- fingerprint.go        # InputShape fingerprinting & tokenization
|   +-- scorer.go             # Confidence scoring & routing
|   +-- *_test.go
|
+-- compile/                  <- MODIFIED: pluggable backend
|   +-- backend.go            # NEW: CompilerBackend interface
|   +-- tiered.go             # NEW: TieredCompiler orchestrator
|   +-- deterministic.go      # NEW: template codegen backend
|   +-- llm.go                # REFACTORED: extract from compiler.go
|   +-- compiler.go           # existing (becomes thin orchestrator)
|   +-- trigger.go            # unchanged
|   +-- gatherer.go           # unchanged
|
+-- tui/                      <- NEW: bubbletea TUI
|   +-- app.go                # Main bubbletea model & program
|   +-- pathlist.go           # Hot paths list view
|   +-- detail.go             # Expanded path detail view
|   +-- diffview.go           # Instance diff view
|   +-- graphview.go          # ASCII graph rendering
|   +-- styles.go             # lipgloss styling
|
cmd/aj/
+-- trace_cmd.go              <- NEW: aj trace subcommand
```

### CompilerBackend Interface

```go
type CompilerBackend interface {
    Name() string
    Compile(ctx context.Context, patterns []trace.Pattern) ([]skills.Skill, error)
}
```

## Configuration Additions

```json
{
  "compile": {
    "deterministic_threshold": 0.6,
    "...existing fields..."
  }
}
```

## Implementation Phases

1. **`internal/trace/`** — Graph builder, fingerprinting, hot path detection, structural diff, scorer. Pure algorithms, no I/O. Heavy unit testing.
2. **`internal/compile/` refactor** — Extract `CompilerBackend` interface, move LLM logic into `llm.go`, wire `TieredCompiler`. Existing behavior unchanged.
3. **`internal/compile/deterministic.go`** — Template codegen backend. Plugs into `TieredCompiler`.
4. **`internal/tui/` + `aj trace`** — Bubbletea TUI. Can be developed in parallel with phases 2-3 (only depends on `internal/trace/`).
5. **Config & CLI wiring** — Add config fields, wire `aj compile` to `TieredCompiler`, add `aj trace` subcommand.
