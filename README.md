```                                         
                                                
                                                
▄████▄  ▄▄▄▄ ▄▄▄▄▄ ▄▄  ▄▄ ▄▄▄▄▄▄   ██ ██ ██████ 
██▄▄██ ██ ▄▄ ██▄▄  ███▄██   ██     ██ ██   ██   
██  ██ ▀███▀ ██▄▄▄ ██ ▀██   ██  ████▀ ██   ██   
                                                                         

the daemon that compiles your workflows into existence
```

> *"We are what we repeatedly do. Excellence, then, is not an act, but a habit."*
> — **Aristotle**, *Nicomachean Ethics*
>
> Your AI agent repeats the same multi-step workflows hundreds of times —
> `kubectl logs`, then `grep`, then `edit`, then `apply` — burning tokens
> on what has already become muscle memory. AJ watches these habits
> form, and in quiet moments of reflection, distills them into instinct.
>
> Like an artisan whose hands move before conscious thought arrives,
> the agent simply *knows* what to do next. Ten thousand tokens of
> deliberation become two hundred tokens of certainty. The stochastic
> becomes deterministic. Repetition becomes skill.

---

## What is AJ?

A background JIT compiler for autonomous coding agents. It operates silently via Claude Code hooks, observes recurring tool-use patterns across sessions, and compiles them into zero-token parameterized skills — no manual configuration required.

```mermaid
flowchart LR
    subgraph hooks["Hook Events"]
        H1(["PostToolUse"])
        H2(["SessionStart"])
        H3(["SessionEnd"])
    end

    subgraph daemon["AJ Daemon"]
        direction TB
        D1["Ingest & Log"]
        D2["Trigger Compile"]
        D3["Emit Skills"]
        D1 --> D2 --> D3
    end

    subgraph skills["Compiled Skills"]
        S1[/"Parameterized"/]
        S2[/"Deterministic"/]
        S3[/"Zero-token"/]
    end

    hooks -- "stdin (JSON)" --> daemon
    daemon -- "~/.aj/skills/" --> skills

    style hooks fill:#4a5568,stroke:#a0aec0,color:#e2e8f0
    style daemon fill:#2b6cb0,stroke:#63b3ed,color:#e2e8f0
    style skills fill:#276749,stroke:#68d391,color:#e2e8f0
```

### The Numbers

| Before | After |
|--------|-------|
| ~10,000 tokens per routine task | ~200 token skill invocation |
| >30s stochastic reasoning | <1s deterministic execution |
| Manual skill authoring | Automatic pattern compilation |

## Supported Agent Harnesses

AJ's ingestion layer is designed to work across multiple agent harnesses. Currently Claude Code is fully supported, with more coming soon.

| Harness | Status | Hook Mechanism | Notes |
|---------|--------|----------------|-------|
| [Claude Code](https://claude.ai/code) | **Supported** | Native hooks (PostToolUse, SessionStart, SessionEnd) | Full support via `aj init` |
| [Codex](https://github.com/openai/codex) | Planned | — | — |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) | Planned | — | — |
| [GitHub Copilot](https://github.com/features/copilot) | Planned | — | — |
| [Cursor](https://cursor.com) | Planned | — | — |

> Want support for another harness? Open an issue.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install agent-jit/tap/aj
```

### Shell Script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/agent-jit/AgentJIT/main/install.sh | sh
```

### Manual Download

Download the latest binary from [Releases](https://github.com/agent-jit/AgentJIT/releases) and place it on your `PATH`:

```bash
# macOS (Apple Silicon)
curl -Lo aj.tar.gz https://github.com/agent-jit/AgentJIT/releases/latest/download/aj_$(curl -s https://api.github.com/repos/agent-jit/AgentJIT/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_darwin_arm64.tar.gz
tar xzf aj.tar.gz aj
sudo mv aj /usr/local/bin/
rm aj.tar.gz

# macOS (Intel)
curl -Lo aj.tar.gz https://github.com/agent-jit/AgentJIT/releases/latest/download/aj_$(curl -s https://api.github.com/repos/agent-jit/AgentJIT/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_darwin_amd64.tar.gz
tar xzf aj.tar.gz aj
sudo mv aj /usr/local/bin/
rm aj.tar.gz

# Linux (x86_64)
curl -Lo aj.tar.gz https://github.com/agent-jit/AgentJIT/releases/latest/download/aj_$(curl -s https://api.github.com/repos/agent-jit/AgentJIT/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_linux_amd64.tar.gz
tar xzf aj.tar.gz aj
sudo mv aj /usr/local/bin/
rm aj.tar.gz
```

**Windows (PowerShell):**

```powershell
$release = Invoke-RestMethod "https://api.github.com/repos/agent-jit/AgentJIT/releases/latest"
$version = $release.tag_name -replace '^v', ''
$url = "https://github.com/agent-jit/AgentJIT/releases/latest/download/aj_${version}_windows_amd64.zip"
Invoke-WebRequest -Uri $url -OutFile aj.zip
Expand-Archive aj.zip -DestinationPath .
Move-Item aj.exe "$env:LOCALAPPDATA\Microsoft\WindowsApps\aj.exe" -Force
Remove-Item aj.zip
```

### Go Install

```bash
go install github.com/agent-jit/agentjit/cmd/aj@latest
```

### Build from Source

```bash
git clone https://github.com/agent-jit/AgentJIT.git
cd AgentJIT
make install    # builds and copies to /usr/local/bin/aj
```

## Quick Start

```bash
# Initialize AJ — creates ~/.aj/, installs Claude Code hooks
aj init

# Or install hooks into a specific project only
aj init --local

# Start the background daemon
aj daemon start

# Trigger a compilation manually
aj compile

# Import historical Claude Code transcripts
aj bootstrap --since 2026-03-01

# View generated skills
aj skills list

# Adjust configuration
aj config get --all
aj config set compile.trigger_mode interval
```

## Architecture

AJ is three loosely-coupled layers in a single Go binary:

```mermaid
flowchart TB
    subgraph cli["CLI · Cobra"]
        direction LR

        subgraph ingestion["Ingestion Layer"]
            direction TB
            I1["Hook stdin"]
            I2["Normalize"]
            I3["JSONL logs"]
            I1 --> I2 --> I3
        end

        subgraph trigger["Trigger Layer"]
            direction TB
            T1["Event count"]
            T2["Interval timer"]
            T3["Manual fire"]
        end

        subgraph compiler["Compiler Layer"]
            direction TB
            C1["Claude Code CLI"]
            C2["Pattern detect"]
            C3["Skill generation"]
            C1 --> C2 --> C3
        end

        ingestion --> trigger --> compiler
    end

    subgraph infra["Infrastructure"]
        direction LR
        U["IPC Transport<br/>(Unix socket / Windows named pipe)"]
        P["PID Lifecycle Mgmt"]
        F["~/.aj/ filesystem"]
    end

    cli --> infra

    style cli fill:#2b6cb0,stroke:#63b3ed,color:#e2e8f0
    style ingestion fill:#2c5282,stroke:#90cdf4,color:#e2e8f0
    style trigger fill:#2c5282,stroke:#90cdf4,color:#e2e8f0
    style compiler fill:#2c5282,stroke:#90cdf4,color:#e2e8f0
    style infra fill:#4a5568,stroke:#a0aec0,color:#e2e8f0
```

**Design philosophy:** The Go binary is a dumb pipe. It handles I/O, lifecycle, and configuration. All intelligence lives in the compiler prompt that Claude executes during compile cycles.

### Data Flow

1. **Ingest** — Claude Code hooks fire on every tool use, piping JSON to `aj ingest` via stdin
2. **Normalize** — Events are normalized into a canonical schema and appended to date/session-partitioned JSONL logs
3. **Trigger** — The daemon monitors event counts or timers and fires the compilation sequence
4. **Compile** — Claude Code reads the logs, identifies recurring multi-step patterns, and generates parameterized skills
5. **Emit** — Skills are written to `~/.aj/skills/` and become available immediately

### Filesystem Layout

```
~/.aj/
├── config.json                 # Configuration with sensible defaults
├── daemon.pid                  # Daemon process ID
├── daemon.sock                 # IPC endpoint (Unix socket; named pipe on Windows)
├── logs/                       # Date/session-partitioned JSONL
│   └── 2026-04-01/
│       └── session_abc123.jsonl
├── skills/                     # Compiled skills (auto-generated)
├── stats.jsonl                 # Token usage and skill execution metrics
├── compile-log.jsonl           # Compiler activity log
└── last_compile_marker         # Timestamp of last compile run
```

## Configuration

Defaults are designed to work out of the box:

```json
{
  "daemon": { "idle_timeout_minutes": 30 },
  "ingestion": { "max_response_bytes": 512, "log_retention_days": 30 },
  "compile": {
    "trigger_mode": "manual",
    "trigger_interval_minutes": 30,
    "trigger_event_threshold": 100,
    "min_pattern_frequency": 3,
    "min_token_savings": 500
  },
  "scope": {
    "global_cli_tools": ["kubectl", "docker", "gh", "aws", "terraform"],
    "cross_project_threshold": 2
  }
}
```

Use dot-notation to get/set any value:

```bash
aj config get compile.trigger_mode
aj config set compile.min_pattern_frequency 5
aj config reset
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `aj init` | Create `~/.aj/`, install hooks, write config |
| `aj init --local` | Install hooks into project-local settings |
| `aj init uninstall` | Remove hooks and optionally delete data |
| `aj daemon start` | Start background daemon |
| `aj daemon stop` | Stop daemon gracefully |
| `aj daemon status` | Show PID, uptime, event count |
| `aj compile` | Manually trigger compilation |
| `aj bootstrap` | Import historical Claude Code transcripts |
| `aj config get [KEY]` | Read config values |
| `aj config set KEY VAL` | Write config values |
| `aj skills list` | List generated skills with ROI stats |
| `aj skills remove NAME` | Remove a compiled skill |
| `aj stats` | Show token usage statistics and ROI |
| `aj stats --json` | Output stats as JSON |
| `aj stats reset` | Clear recorded statistics |
| `aj ingest` | Internal: receive hook JSON from stdin |

## Development

```bash
make build        # Build binary to ./aj
make test         # Run all tests
make clean        # Remove build artifacts
make install      # Install to $GOPATH/bin
```

**Requirements:** Go 1.22+

## How It Works — The Compile Cycle

When the daemon triggers a compile (by event threshold, timer, or manual invocation), it orchestrates a reflection cycle:

1. **Gather** — Collect JSONL logs since the last compile marker
2. **Analyze** — Pass logs to Claude with the compiler prompt
3. **Identify** — Claude detects recurring multi-step tool-use patterns (≥3 occurrences)
4. **Parameterize** — Variable parts (file paths, namespaces, pod names) become parameters
5. **Evaluate** — Calculate token savings; reject patterns below the `min_token_savings` threshold
6. **Compile** — Generate deterministic skill files with metadata and ROI tracking
7. **Register** — Skills become immediately available for future sessions

The agent doesn't learn to do new things. It learns to stop *thinking* about things it already knows how to do.

## License

MIT

---

<p align="center">
<i>"The energy of the mind is the essence of life."</i> — Aristotle
</p>
