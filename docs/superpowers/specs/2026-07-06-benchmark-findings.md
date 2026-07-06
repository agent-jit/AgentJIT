# AgentJIT Benchmark — Findings (v1)

**Status:** Initial results
**Date:** 2026-07-06
**Harness:** `aj bench` (see `docs/superpowers/specs/2026-07-05-benchmark-sandbox-design.md`)
**Agent under test:** Claude Code CLI (`claude` v2.1.179), model claude-opus-4.8, temperature 0

---

## TL;DR

On the first repetition-parameterized workflow (`nullcheck`), a compiled skill produced a
**~0.15% token saving** (354 tokens out of ~236k) at equal (100%) success. With any realistic
compile cost, **break-even is thousands of invocations — i.e. not worth compiling for this shape.**

This is a *useful* result, not a disappointing one: the benchmark exists to answer "does a skill
actually save tokens, and after how many uses does it pay off?" — and here the honest answer is "no,
not on a trivial edit." The methodology (measure at iso-accuracy from API-reported usage, never
estimate) is what makes that answer trustworthy.

## Method (as implemented)

- **Two arms**, identical except the skill surface:
  - `baseline` — the agent solves the task cold.
  - `jit` — a project-local `.claude/skills/<name>/SKILL.md` (the deterministic transform) is installed
    first, modelling "compiled from prior runs".
- **Fresh fixture per arm.** The baseline arm mutates the working tree, so the JIT arm regenerates a
  clean copy — otherwise the comparison is silently corrupted.
- **Tokens-to-Success (T2S)** = total context tokens (input + output + cache) to reach a *verified*
  result, read from `claude`'s reported `usage`. Never estimated.
- **Iso-accuracy gate.** A rollout counts toward T2S only if it passes the task's verifier. A skill that
  lowers success rate cannot bank "savings" — it would just be failing more cheaply.
- **Dual-gate verifier** for `nullcheck`: the package must build *and* exactly N guards must be present
  (grep scoped to `*.go`), catching "did something, but not the right thing".

## Result — `nullcheck` (n = 2)

```
aj bench --gen nullcheck --n 2 --compare
  nullcheck-2:  baseline 100% / jit 100%   (iso-accuracy)
    T2S baseline 235,695  vs  jit 235,341   →  saving 354 tokens/use
```

Both arms verified (2 nil-guards added, package builds). Earlier single-arm baseline sweep:

```
aj bench --gen nullcheck --n 1 --arm baseline   T2S 280,752  (1 guard, verified)
aj bench --gen nullcheck --n 2 --arm baseline   T2S 235,999  (2 guards, verified)
```

## Interpretation

- **Baseline T2S is dominated by fixed overhead**, not the task. A trivial `--print` prompt already
  costs ~44k cache-creation tokens (system prompt + tool defs). The actual edit is a rounding error, so
  a skill that only shortcuts the edit saves almost nothing.
- **Break-even scales with the saving.** `break-even ≈ compile_cost / per_use_saving`. At 354 tokens
  saved/use, even a modest compile cost implies thousands of uses to recoup — this workflow should
  **not** be compiled.
- **This is the intended signal.** Per the design, the headline is amortized break-even *stratified by
  shape*, not a global average. `nullcheck` is one (low-value) point on that curve.

## Where a skill *should* pay off (hypothesis)

JIT value comes from skipping **exploration**, not from shortening a known edit. `nullcheck` needs
almost no exploration, so there is nothing to save. A skill should show a real delta on workflows where
the baseline burns tokens *finding* what to change — e.g. `migrate-N`: locate every call site of an API
across N files, then apply a mechanical change. That is the natural next fixture.

## Reproducing

```
# isolated sandbox; real data untouched
AJ_HOME=$(mktemp -d) aj bench --gen nullcheck --n 1,2 --compare --rollouts 3
```

Flags: `--tasks | --gen`, `--arm baseline|jit`, `--compare`, `--rollouts`, `--n` (curve),
`--compile-cost` (auto-read from `$AJ_HOME` stats if unset), `--dry-run`, `--json`.

## Limitations / next steps

- Single trivial shape so far; add `migrate-N` (exploration-heavy) to find where JIT pays off.
- Skill is a hand-written project-local SKILL.md, not one produced by `aj compile` from real logs;
  a follow-up should close that loop so compile cost is measured from an actual compilation.
- Rollouts here were 1 for cost/time; production numbers should average N≥3 and report the spread
  (the harness already computes mean/median/min/max).
- Phase 4 (replay real `aj bootstrap` transcripts) remains, as an external-validity check.
