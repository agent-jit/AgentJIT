# Changelog

All notable changes to AgentJIT will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.2]: https://github.com/agent-jit/AgentJIT/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/agent-jit/AgentJIT/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/agent-jit/AgentJIT/releases/tag/v0.1.0
