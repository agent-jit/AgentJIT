# Changelog

All notable changes to AgentJIT will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.7] - 2026-04-10

### Added
- Auto-compile trigger in daemon: evaluates `ShouldFire()` on a 10s ticker for `event_count` and `interval` trigger modes
- Separate `compileEventCount` atomic counter tracks events since last compile independently from total event count
- "Next Compile" section in `aj stats` output showing trigger mode, progress toward next compilation (remaining events or time), and last compile timestamp
- `CountEventsSinceMarker` lightweight event counter reads log files without full deserialization
- JSON output for `aj stats --json` now includes `next_compile` field

### Fixed
- `EventsSinceCompile()` now correctly tracks events since last compile rather than returning total count
- PowerShell companion scripts now detect native command failures via `PSNativeCommandUseErrorActionPreference`

### Changed
- Renamed binary from `agentjit` to `aj` (`cmd/agentjit` → `cmd/aj`), updated goreleaser, Makefile, and README

## [0.1.6] - 2026-04-10

### Added
- Platform-aware skill compilation: compiler prompt receives `{{PLATFORM}}` and `{{SHELL}}` variables, generating `.ps1` PowerShell scripts on Windows instead of `.sh` bash scripts
- Compile now scopes Claude's access to only post-marker log directories, preventing re-analysis of already-compiled sessions

### Fixed
- `copySkillDir` now recursively copies subdirectories so Windows fallback copy path includes companion scripts
- Remaining errcheck lint errors in cmd, watcher test, bootstrap, and test files

## [0.1.5] - 2026-04-06

### Fixed
- Detach daemon stdio to prevent SessionStart hook hang

## [0.1.4] - 2026-04-06

### Added
- `aj stats record` subcommand for script-based skill execution tracking

## [0.1.3] - 2026-04-03

### Added
- TRIGGER clauses in compiled skill descriptions for auto-invocation via the Skill tool
- Install script (`install.sh`) and Windows download instructions in README

### Fixed
- Unchecked error return values to satisfy errcheck linter
- Unsupported `allowed-tools` flag replaced with portable skill paths
- Claude subprocess now killed on Ctrl+C with interrupt marker

## [0.1.2] - 2026-04-03

### Added
- Token consumption metrics tracking for compile sessions (`aj stats`)
- `aj stats` command with dashboard, `--json` output, and `reset` subcommand
- Skill execution tracking via PostToolUse hooks (success/failure, estimated tokens saved)
- Stats storage as append-only JSONL at `~/.aj/stats.jsonl`
- Skill discovery via symlinks from `~/.aj/skills/` into `~/.claude/skills/`
- Automatic skill linking during compilation, daemon skill watch, and `aj init`
- Symlink fallback to file copy for Windows compatibility

### Changed
- `aj compile` now uses `--output-format json` to capture token usage from Claude CLI
- `aj compile` prints the result text and a token usage summary after compilation
- `aj init` adds a new step to sync skill symlinks to Claude Code
- `aj skills remove` also removes the corresponding symlink from Claude Code

### Fixed
- Compile trigger_mode validation to reject unknown values

## [0.1.1] - 2026-04-02

### Added
- Homebrew tap support via goreleaser brews configuration
- Windows support via platform-split IPC (named pipes) and process management

## [0.1.0] - 2026-04-02

### Added
- Initial release of AgentJIT (`aj`)
- Background daemon with Unix socket IPC for event collection
- Claude Code hook integration (PostToolUse, PostToolUseFailure, SessionStart, SessionEnd)
- Event normalization and JSONL log storage with date/session partitioning
- JIT compilation via `aj compile` using Claude Code CLI for pattern detection
- Manifest-based log navigation for memory-efficient compilation
- Skill generation with SKILL.md, metadata.json, and companion shell scripts
- ROI estimation for generated skills (savings per invocation, projected savings)
- `aj init` for setup and hook installation (global and project-local)
- `aj config get/set/reset` for configuration management
- `aj skills list/remove` for skill inventory management
- `aj bootstrap` for importing historical Claude Code transcripts
- Compile triggers: manual, interval-based, and event-count-based
- Idle timeout auto-shutdown for the daemon
- Log retention with configurable cleanup period
- Scope inference (global vs. local skills) based on working directories
- fsnotify-based skill file watcher with daemon notifications
- CI pipeline with goreleaser for cross-platform builds (linux, darwin, windows)

[0.1.3]: https://github.com/agent-jit/AgentJIT/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/agent-jit/AgentJIT/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/agent-jit/AgentJIT/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/agent-jit/AgentJIT/releases/tag/v0.1.0
