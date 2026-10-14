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
