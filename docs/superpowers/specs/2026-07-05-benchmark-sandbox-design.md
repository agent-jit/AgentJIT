# AgentJIT Benchmark Sandbox — Design Specification

**Status:** Draft (pending @Ilya_Sutskever review)
**Date:** 2026-07-05
**Authors:** @Agents-dev (owner), @Ilya_Sutskever (methodology)
**Tech Stack:** Go (`aj bench` subcommand), reuses `internal/stats` + `internal/config`

---

## 1. Overview

A reproducible sandbox that measures whether AgentJIT's compiled skills actually save tokens, and
after how many repetitions they pay for themselves. It runs a suite of agent tasks under two arms —
**BASELINE** (no skills) and **JIT** (skills pre-compiled) — holding everything else fixed, and reports
the results at **iso-accuracy** (savings only count when the task is verifiably solved).

### The question

"Skills save tokens" is asserted but rarely measured. Because AgentJIT is literally a JIT compiler,
its cost structure is a classic amortization problem: compiling a skill spends tokens up front, and
each subsequent invocation saves. The decision-relevant number is therefore not "% saved per use" but
**break-even: how many repetitions until the skill recoups its compile cost.**

### Non-goals

- Not a general agent-eval framework. Scope is AgentJIT skill ROI.
- Not a wall-clock benchmark first (latency is a secondary metric; tokens/accuracy lead).

---

## 2. Methodology (adopted from @Ilya_Sutskever's LSP-vs-grep study)

Reference: `github.com/Poytr1/lsp-vs-grep-token-study` — a near-identical token-efficiency study whose
harness structure (agent loop, real-execution verifiers, arm design) we port.

### 2.1 Primary metric — Tokens-to-Success (T2S)

Total context tokens consumed to reach a **verified** result, counted from the **API-reported usage**
(never from AgentJIT's `EstimatedTokensSaved`, which is a model, not a measurement).

### 2.2 Iso-accuracy gate (the #1 trap)

A rollout contributes to token statistics **only if it passes the verifier** (tests green / gold-patch
match). A skill that lowers success rate does not get to bank "savings" — otherwise it is merely
*failing more cheaply*. Every arm reports success rate alongside tokens; comparisons are made at equal
success only.

### 2.3 Determinism recipe

- Fixed model, **temperature = 0** (greedy).
- Task state pinned by **git SHA**.
- **Skill/tool description text length held constant across arms** — description length is itself a
  context-token variable and a behavior nudge; it must not confound the arms.
- Residual nondeterminism averaged over **N = 3 rollouts** per (task, arm).

### 2.4 Arms

| Arm | Skills dir | What the agent does |
|-----|-----------|---------------------|
| **BASELINE** | empty | Solves each task cold |
| **JIT** | pre-compiled skills for that workflow present | Solves the same task with skills available |

Only the skill surface varies. Model, tasks, prompt, loop, and sandbox layout are identical.

---

## 3. Task corpus

Two tiers, reported separately and honestly.

### 3.1 Pipeline smoke test — reuse `rename_tasks.jsonl`

Ilya's verifier-backed rename fixtures are used **only to wire up and validate the harness pipeline**
(known-good, de-risks the port). **Caveat (his):** rename is a *poor* JIT candidate — each rename
targets a different symbol, so the compiled skill does not transfer to the next task. Rename tasks lack
the reuse axis that gives JIT its value. They validate plumbing; they are **not** the measurement.

### 3.2 Primary measurement — AgentJIT-native, repetition-parameterized tasks

Author a small batch of **same-shape, reusable** workflows parameterized by repetition count (shape ×N),
e.g.:
- "Add a null-check to every function matching pattern P" (×N functions/files).
- "Apply the same migration transform to N files."
- A canonical `grep → read → edit → test` loop repeated over N structurally-identical targets.

These are the real fuel for the break-even curve: the *same* compiled skill is exercised N times.

---

## 4. Headline result — amortized break-even

For each workflow shape:

- **compile_cost** = tokens spent building the skill. AgentJIT already records this separately in
  `internal/stats`: `CompileSessionData.{InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens}`.
- **per_invocation_saving** = `T2S(BASELINE) − T2S(JIT)` at iso-accuracy, per single invocation.
- **break-even repetitions** = `compile_cost / per_invocation_saving`.

Reporting:

1. **Break-even curve keyed on repetition count** — the primary deliverable. **NOT a single global
   mean** (a universal average lies).
2. **Stratify BOTH break-even AND compile_cost by workflow shape** — compile costs vary widely by
   shape; a global average hides the "high-compile-cost, low-reuse" trap.
3. Per-workflow **token-reduction %** = the per-use benefit feeding the curve (secondary).
4. **ROI multiplier** — derived, tertiary.

Conceptual parallel (his): skills amortize only on *repeated* workflows, exactly as code-actions
amortize only on *multi-step* tasks. The benchmark's job is to make that conditional structure visible,
not to collapse it to one number.

---

## 5. Sandbox isolation

Each arm runs against an isolated AgentJIT root so real `~/.aj` is never touched.

- `config.PathsFromRoot(root)` already supports an arbitrary root (used by tests); the core supports
  isolation, the CLI just doesn't expose it.
- **First PR:** add an `AJ_HOME` env var (and/or `--root` flag) honored by `config.DefaultPaths()` so
  every `aj` subcommand can target a sandbox root. Small, generally useful beyond benchmarking.
- Compiled skills install into the agent's skill dir; the arm toggle is *whether those skills are
  present*. BASELINE = empty skills dir; JIT = skills compiled from prior runs present.

---

## 6. Implementation — `aj bench` (Go, in-repo)

**Decision: Go subcommand, not a standalone Python harness** (even though the reference is Python).
Rationale: the token-counting source of truth lives in Go (`internal/stats`); a Python shell would
duplicate that logic and drift. Port only the language-agnostic methodology skeleton (arms, T2S,
verifier gate).

Proposed layout:

- `cmd/aj/bench_cmd.go` — `aj bench` driver (cobra), flags for task file, arms, N rollouts, sandbox root.
- `internal/bench/` — harness (agent episode loop), arm setup/teardown, verifier interface, aggregation.
- `bench/tasks/` — task fixtures (`rename_smoke.jsonl` + AgentJIT-native repetition tasks).
- Reuses `internal/stats` for token accounting and `internal/config.PathsFromRoot` for isolation.
- Output: JSONL per-rollout records + a summary report (break-even curve data, stratified).

Verifier interface (ported concept from `verify_rename.py` / `verify_edit.py`): each task declares how
to check a solved result (run tests / diff against gold); only verified rollouts count toward T2S.

---

## 7. Phased delivery

- **Phase 0 — Isolation flag.** `AJ_HOME`/`--root`; unit test that two roots don't collide. (First PR.)
- **Phase 1 — Harness skeleton.** `aj bench` runs one task, one arm, N rollouts; records API-reported
  tokens via `internal/stats`; verifier gate. Validate on Ilya's rename smoke fixtures.
- **Phase 2 — Two-arm A/B + aggregation.** BASELINE vs JIT; T2S at iso-accuracy; per-task report.
- **Phase 3 — Repetition tasks + break-even.** AgentJIT-native shape-×N fixtures; break-even curve,
  stratified by shape; compile_cost measured separately.
- **Phase 4 — Real-transcript sanity.** Small batch of `aj bootstrap`-replayed real workflows as an
  external-validity check; report beside synthetic.

---

## 8. Open items

- Exact set of Phase-3 repetition-parameterized task shapes (author list — TBD, small batch first).
- Whether to gate `aj bench` behind a build tag / keep fixtures out of the release binary.
- Reviewer: @Ilya_Sutskever (methodology), @pchsu (scope + headline framing).
