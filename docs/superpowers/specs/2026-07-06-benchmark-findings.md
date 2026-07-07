# AgentJIT Benchmark — Findings (v1)

**Status:** Initial results
**Date:** 2026-07-06
**Harness:** `aj bench` (see `docs/superpowers/specs/2026-07-05-benchmark-sandbox-design.md`)
**Agent under test:** Claude Code CLI (`claude` v2.1.179), model claude-opus-4.8, temperature 0

---

## TL;DR

On two trivial repetition workflows — `nullcheck` (hand-written skill) and `shellseq` (a **real
`aj compile`** skill) — a skill produced a **~0% token saving** (354 and 19–35 tokens on ~236k / ~93k
baselines) at equal (100%) success. With any realistic compile cost, **break-even is thousands of
invocations — i.e. not worth compiling for these shapes.**

This is a *useful* result, not a disappointing one: the benchmark exists to answer "does a skill
actually save tokens, and after how many uses does it pay off?" — and here the honest answer is "no,
not on a trivial edit or a short command sequence." It holds for a genuinely compiled skill, not just a
hand-written stand-in. The methodology (measure at iso-accuracy from API-reported usage, never estimate)
is what makes that answer trustworthy. The open question is whether an **exploration-heavy** workflow —
where the baseline burns tokens *finding* what to change — flips the result.

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

## Update — real `aj compile` skill (`shellseq`, 2026-07-07)

The `nullcheck` result above used a *hand-written* skill. To test AgentJIT's own compiler, the `shellseq`
fixture uses a repetitive shell workflow (`mkdir -p out` + N `touch`) that the **deterministic** compiler
turns into a real skill: the JIT arm seeds session logs, runs the actual `aj compile`, and installs the
generated `mkdir-out/scripts/mkdir-out.sh`. Compile cost is **0** (deterministic path is zero-token).

Measured at **`--rollouts 3`** (mean ≈ median, so the single-rollout cache noise is gone):

```
shellseq n=2:  baseline 92,729 (med 92,704)  vs  jit 92,694 (med 92,694)  →  saving  35/use
shellseq n=4:  baseline 92,808 (med 92,810)  vs  jit 92,789 (med 92,792)  →  saving  19/use
```

Both arms 100% at iso-accuracy. **The genuinely compiled skill saves ~0% (19–35 tokens on a ~92.7k
baseline)** — even less than the hand-written `nullcheck` skill (354), and the saving *shrinks* as N
grows. This confirms the conclusion holds for a **real** compiled skill, not just a stand-in: on a short
known command sequence there is nothing to save, because baseline T2S is dominated by fixed context load.

**Only Bash-shaped workflows are deterministically compilable** — the deterministic backend emits only
Bash steps, so an Edit-based workflow like `nullcheck` routes to the LLM compiler instead (which is why
`nullcheck` keeps a hand-written skill here).

**Caveat on single-rollout runs:** baseline T2S is dominated by prompt-cache state that carries across
the two sequential episodes, so `--rollouts 1` gives unreliable A/B numbers (a run once showed baseline
0 vs jit 46k). Always use `--rollouts >= 3`; the harness reports mean/median/min/max.

## Reproducing

```
# isolated sandbox; real data untouched
AJ_HOME=$(mktemp -d) aj bench --gen nullcheck --n 1,2 --compare --rollouts 3   # hand-written skill
AJ_HOME=$(mktemp -d) aj bench --gen shellseq  --n 2,4 --compare --rollouts 3   # real aj compile skill
```

Flags: `--tasks | --gen`, `--arm baseline|jit`, `--compare`, `--rollouts`, `--n` (curve),
`--compile-cost` (auto-read from `$AJ_HOME` stats if unset), `--dry-run`, `--json`.

## Limitations / next steps

- Two trivial shapes so far (`nullcheck`, `shellseq`), both ~0% saving. Add an **exploration-heavy**
  shape (e.g. `migrate-N`: locate every call site across N files, then apply a mechanical change) — the
  hypothesis is that JIT only pays off where the baseline burns tokens *finding* what to change.
- **Addressed 2026-07-07:** the JIT skill can now come from real `aj compile` (`shellseq`), and results
  are reported at `--rollouts 3`. Still open: only *deterministic* (Bash) workflows are compiled this way;
  an LLM-compiled skill (for Edit-based workflows) would have a real, non-zero compile cost to measure.
- Phase 4 (replay real `aj bootstrap` transcripts) remains, as an external-validity check.
