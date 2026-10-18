# Benchmark accounting and isolation

Each arm snapshots `repo_dir` (the current directory if omitted). Its setup
runs once on the snapshot; each rollout then receives a fresh copy. The agent
and verifier use that same copy, which is removed afterwards. Original task
files are not the working directory. Relative fixture paths are required for
isolation: absolute paths in prompts or verification commands, external
services, and global caches are not sandboxed. Internal symlinks are relocated;
external symlinks are rejected rather than silently sharing mutable files.

Successful-only T2S distributions remain diagnostic. Savings use the mean token
count across **all attempts**, including unsuccessful verification and agent
errors with reported usage. JSON comparisons also include each arm's total
tokens and reported USD cost. Missing usage blocks savings and break-even
claims; it is not treated as a free attempt. Such totals may be incomplete.
Savings and break-even require verified successes in both arms and equal
observed success rates. Equal observed rates are not statistical evidence of
equivalent accuracy; repeat runs and inspect uncertainty before drawing a
performance conclusion.

Break-even is a token estimate using the supplied compile cost, not a dollar
ROI or a complete lifecycle-cost measurement. Trace collection, external
validation, tool charges, and retries outside the measured episode must still
be accounted for separately. Do not interpret zero deterministic compilation
tokens as zero total setup cost.

Shell parameterization preserves identical command source. For differing
commands it currently supports literal simple-command arguments and emits
quoted substitutions. Differing commands involving expansions, operators, or
incompatible argument counts are declined as a whole pattern. This deliberately
reduces coverage rather than emitting a misleading first-example script; it
does not add runtime fallback or certify PowerShell equivalence.

Regression checks (Go 1.24+; Bash for generated-script execution):

```sh
go test ./internal/bench ./internal/trace ./internal/compile
go test ./...
```

## Latency and reliability

`aj bench` now measures monotonic wall-clock time around each agent call and
verification, independently of Claude's existing self-reported
`usage.duration_ms`. Per-rollout `agent_ms`, `verify_ms`, and `wall_ms` are in
milliseconds; wall time includes agent + verifier, but excludes fixture copying,
setup/compilation, cleanup and result fingerprinting. The once-per-arm
`setup_ms` includes seed copying and skill preparation/compilation. It is not
amortized away or a claim about cold-start latency. No tool-only latency is
inferred from total agent time.

Comparisons preserve the full `baseline_result` and `jit_result` including raw
attempts and setup costs. `baseline_metrics` / `jit_metrics` include:

- All-started-attempt and successful-only wall latency: sample count, mean,
  nearest-rank p50/p95, min/max. A failed attempt still contributes latency.
  An attempt blocked by setup/isolation has no timing, never a zero sample.
- Agent and verifier latency distributions, separately.
- Verified successes / all requested attempts, success rate and Wilson 95%
  interval. Setup failures remain failures in this denominator.
- Failure counts: `setup_error`, `isolation_error`, `agent_error`,
  `verifier_error`, `verification_failed`, `timeout`, `canceled` (or `unknown`
  for legacy records). Raw error strings are retained. Verifier nonzero exits
  are `verification_failed`, not inferred model/tool root causes.
- Mixed pass/fail outcomes, distinct successful result fingerprints and pairwise
  result agreement (identical pairs / all fingerprinted successful pairs).
  Agreement is absent with fewer than two fingerprints; coverage is explicit.
  All-identical failures cannot count as stable success.

Optional task `result_files` lists relative regular files defining semantic
output. The SHA-256 digest covers file names and bytes, not incidental agent
prose, timestamps, or the injected skill. Shellseq uses its exact empty marker
files; aksops uses the exact ordered command audit log, excluding randomized pod
stdout. Digest errors remain separate from correctness failures and do not
silently change verifier results. Custom tasks without a projection report
success/failure stability only. A stable digest is evidence only about the
chosen projection, not proof of deterministic model behavior.

Both generated arms have identical prompts, verifiers and result projections;
`RunPaired` rejects mismatches. Shellseq verifies correct names and empty content
in addition to marker count. Aksops now verifies exact arguments, order and
multiplicity (the old grep verifier did not actually enforce order). Historical
results are **not** revalidated by this change.

Comparisons prepare each arm once and alternate rollout order AB, BA, AB, BA.
Each attempt still gets a fresh isolated directory. This reduces fixed arm-order
bias but does not isolate model/service state, OS caches or prompt caches.
The default `--timeout 10m` bounds each agent + verification attempt; use the
same limit for both arms. Timeouts are censored failures, not completed tasks.
`wall_mean_saving_ms` is emitted only for equal-sized arms with complete timing,
verified successes and equal observed success rates, independently of token
usage availability. It is descriptive, **not** statistical evidence of a win.

### Repeated real-model protocol

Start with at least 30 rollouts **per arm, per task**, and repeat the whole
experiment in multiple independent sessions with fresh workdirs/AJ_HOME. The
CLI warns below 20; this is a small-sample warning, not a significance threshold.
Even at n=30, nearest-rank p95 is driven by the two slowest observations, and
30/30 successes has a Wilson lower bound of only about 88.6%. Increase sample
size before reliability or tail-latency claims. Do not stop early after a win.

```sh
go build -o /tmp/aj ./cmd/aj
# Set MODEL to the same explicit model ID in both experiments.
AJ_HOME=$(mktemp -d) /tmp/aj bench --gen shellseq --n 2 --compare \
  --model "$MODEL" --rollouts 30 --timeout 10m --json > shellseq.json
AJ_HOME=$(mktemp -d) /tmp/aj bench --gen aksops --n 2 --compare \
  --model "$MODEL" --rollouts 30 --timeout 10m --json > aksops.json
```

Seed sessions use n=2, but the current shellseq compiler selects the
`mkdir + first touch` prefix: its `mkdir-out` script alone does not complete
shellseq n=2. The real agent must execute the remaining step for the verifier
to pass. This is measured task completion, not automatic evidence of a compiler
speedup. Include shellseq n=1 as a complete-script control; sweeping other n
also tests skill generalization. Record repository SHA, Claude CLI
version, model ID, machine, start time, timeout and cache policy alongside the
JSON. Generated path messages and small-sample warnings go to stderr so stdout
is valid JSON. `--tasks --compare` still requires the caller to configure the
arm-specific skill surface; use `--gen --compare` for built-in real compilation.

Inspect success intervals/failures before latency, then report both all-attempt
and successful-only p50/p95 with sample counts. Analyze paired raw samples and
uncertainty across independent sessions; equal observed rates do not establish
non-inferiority. Include setup costs when discussing amortized wall-clock value.
A lower mean from more early failures is not a latency win. Mock aksops measures
local orchestration, not real AKS/network reliability.

`TestRepeatedCompiledFixtures` runs real compilation and 20 deterministic
scripted attempts per arm for shellseq n=1/n=2 and aksops n=2, with identical
verification and result fingerprints. Complete-script controls must succeed;
the current incomplete shellseq n=2 script must fail all JIT attempts and be
ineligible for a latency claim. It validates the measurement plumbing without an API
key. It **does not** measure a real model or demonstrate JIT benefits. Existing
token findings remain unchanged until real-model measurements provide evidence.
