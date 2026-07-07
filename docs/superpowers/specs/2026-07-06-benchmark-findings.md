# AgentJIT Benchmark — Findings (v1)

**Status:** Initial results
**Date:** 2026-07-06
**Harness:** `aj bench` (see `docs/superpowers/specs/2026-07-05-benchmark-sandbox-design.md`)
**Agent under test:** Claude Code CLI (`claude` v2.1.179), model claude-opus-4.8, temperature 0

---

## TL;DR

**Whether a compiled skill saves tokens depends on the *kind* of workflow — and the split is sharp.**
Across four repetition workflows at 100% iso-accuracy:

| shape | kind | skill source | per-use token delta |
|---|---|---|---|
| `nullcheck` | code-edit | hand-written | ~0% (−354 on ~236k) |
| `shellseq` | trivial shell | real `aj compile` | ~0% (−19..35 on ~93k) |
| `migrate` | code-edit (exploration) | hand-written | **−25% (JIT ~48k WORSE)** |
| `aksops` | **SRE / tool-use** | **real `aj compile`** | **+14% (JIT ~47k BETTER)** |

**The rule:** a **code-edit** skill can only *annotate* — the model still has to read and edit the files
itself, so the skill is pure added context cost (net-negative on `migrate`). An **ops / tool-use** skill
compiles to a **runnable script** the agent *invokes to do the work* (run the command sequence) instead
of exploring and reasoning through each step — that genuinely replaces work, and it pays off (`aksops`,
+14%).

**So "repetitive workflow → compile a skill" is not a safe default.** AgentJIT should preferentially
compile repetitive **Bash / tool-use sequences** (the SRE runbooks its `GlobalCLITools` list already
targets — `kubectl`, `az`, `docker`, `aws`, …) and be skeptical of code-edit patterns. The value is in
replacing *execution*, not annotating *edits*.

This is what the benchmark exists for: a trustworthy, measured answer (iso-accuracy, tokens from
API-reported usage, never estimated) to "does a skill actually save tokens, and where?" — including the
counterintuitive parts (a skill can make things *worse*).

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

## Where a skill *should* pay off (hypothesis — since TESTED, see `migrate` below)

JIT value should come from skipping **exploration**, not from shortening a known edit. `nullcheck` needs
almost no exploration, so there is nothing to save. The hypothesis was that a skill would show a real
delta on workflows where the baseline burns tokens *finding* what to change — e.g. `migrate-N`: locate
every call site of an API across N files, then apply a mechanical change.

**This hypothesis was tested with the `migrate` fixture (below) and did NOT hold** — injecting the skill
made it *worse*, not better. See "Update — exploration-heavy `migrate`".

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

## Update — exploration-heavy `migrate` (2026-07-07): JIT made it WORSE

`migrate` is the shape the hypothesis predicted a win for: N Go files each call a deprecated `OldName`
buried among distractor helpers, and the task is to migrate every `OldName` call to `NewName`. The agent
must **search/read across files** to find the call sites before editing — the exploration a skill was
meant to remove. (Edit-based, so the JIT arm uses a hand-written skill naming the exact rename.)

Measured at `--rollouts 3`:

```
migrate n=3:  baseline 190,635 (med 190,036)  vs  jit 238,512 (med 238,183)  →  saving −47,877/use
```

Both 100% at iso-accuracy (both migrated correctly). **The JIT arm was ~25% WORSE, not better** —
consistent across rollouts (mean ≈ median). The hypothesis is refuted for this configuration.

**Why:** injecting the skill *adds* context tokens, and the agent still reads the files to apply the
edits — so the skill did not remove the exploration; it piled overhead on top of it. A naive
project-local skill can be **net-negative even on an exploration-heavy task**.

**Cross-shape summary (all at iso-accuracy):**

| shape | skill source | per-use token delta |
|---|---|---|
| `nullcheck` | hand-written | ~0% (−354 tok on ~236k) |
| `shellseq` | real `aj compile` | ~0% (−19..35 tok on ~93k) |
| `migrate` | hand-written | **+25% (JIT ~48k MORE)** |

**Takeaway:** injecting a skill is not free, and for these Claude-Code-shaped tasks the skill's context
cost tends to swamp what it saves — sometimes badly. "Repetitive workflow → compile a skill" is *not*
a safe default; the value case has to be targeted much more carefully (bigger per-episode work, or a
skill that genuinely removes reading, not just guides it).

## Update — SRE runbook `aksops` (2026-07-07): JIT PAYS OFF (+14%)

`aksops` is the shape AgentJIT was actually designed for: a repetitive **operations runbook** —
`az aks get-credentials` then `kubectl scale` / `rollout status` / `get pods`. It is Bash-shaped, so the
**deterministic** compiler produces a real *runnable* skill script, and the JIT arm invokes it to *run
the commands* rather than reasoning through each. (`az`/`kubectl` are mocked under `bin/` so the
benchmark is hermetic — no cluster.)

Measured at `--rollouts 3`, JIT skill from **real `aj compile`**:

```
aksops n=3:  baseline 332,620 (med 332,565)  vs  jit 285,391 (med 332,996)  →  saving +47,228/use (~14%)
```

Both 100% at iso-accuracy (both ran the full runbook). **This is the first positive result** — and it is
the direct consequence of the mechanism found in the "why" investigation below: an ops skill *replaces*
work, a code-edit skill only *annotates* it.

*Caveat:* the JIT mean (285k) is below its median (333k) → real rollout variance; the **+14% direction is
solid** but a tight figure needs more rollouts.

## Why a code-edit skill doesn't save (traced)

A JIT-arm `migrate` episode traced with `claude --output-format stream-json --verbose` did, in order:

```
1. Grep OldName   2. Glob **/*.go   3. Skill(migrate-oldname)   4-7. Read ×4   8-10. Edit ×3   11. Grep   12. Bash build
```

Three things make the skill pure overhead here: (1) the agent **explores before it opens the skill**
(steps 1-2 precede step 3), so the skill can't prevent exploration that already happened; (2) it **reads
every file anyway** (4-7) even though the skill named the exact lines — the skill *guided* but didn't
*replace* reading; (3) invoking the Skill **loads its content into context** (added cost). Net: the JIT
arm does everything the baseline does *plus* the skill → strictly more tokens.

The ops case escapes all three because the skill is a **script to execute**, not a description to read.

## A note on hermetic mocks changing behavior

An early `aksops` run **failed**: the agent inspected the mock `az`/`kubectl`, realized they were stubs,
and *refused to run them* ("they don't talk to any cluster … I'll pause here"). The fix was to make the
mocks print realistic CLI output and phrase the task mechanically ("execute exactly as written"). Lesson:
a too-obviously-fake hermetic mock can change what the agent decides to do, and skew a benchmark.

## Reproducing

```
# isolated sandbox; real data untouched
AJ_HOME=$(mktemp -d) aj bench --gen nullcheck --n 1,2 --compare --rollouts 3   # hand-written skill
AJ_HOME=$(mktemp -d) aj bench --gen shellseq  --n 2,4 --compare --rollouts 3   # real aj compile skill
AJ_HOME=$(mktemp -d) aj bench --gen migrate   --n 3   --compare --rollouts 3   # exploration-heavy (JIT worse)
AJ_HOME=$(mktemp -d) aj bench --gen aksops    --n 3   --compare --rollouts 3   # SRE runbook (JIT +14%)
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
