# 🏛️ ArchGuard Analyzer

> **AI-powered architectural analyzer for layered applications.**  
> Finds violations in layer dependencies, domain rules, and design principles — using your LLM as the reasoning engine.  
> 🔒 **Source code never leaves your machine** — the LLM reads only what it explicitly asks for, through a controlled consent layer.

---

## ✨ Features

- 🗂️ **Layer dependency checks** — Domain must not depend on Infrastructure, etc.
- 📜 **Domain-specific rules** — custom semantic rules defined in plain YAML
- 🌐 **Multi-language** — PHP, TypeScript, Python, Java, C#, Go, JavaScript (auto-detected)
- 🤖 **Three LLM providers** — Anthropic Claude, Google Gemini, OpenAI GPT
- 🔐 **Consent system** — interactive prompts before reading any file; decisions persist per session or project
- 📋 **Audit log** — every tool call, decision, and token count recorded in the report
- ⏯️ **Resume mode** — continue an interrupted or incomplete analysis from a previous report
- 📊 **Reports** — structured JSON + human-readable Markdown
- 🌍 **External context** — optionally query Notion, Jira, or YouTrack for domain knowledge before analysis

---

## 📋 Prerequisites

| Requirement | Version | Check |
|---|---|---|
| **Go** | 1.24+ | `go version` |
| **C compiler** (clang or gcc) | any recent | `clang --version` — needed for tree-sitter language detection |
| **Node.js** (optional) | 18+ | needed only for Notion / YouTrack MCP servers |
| **Python / uv** (optional) | — | needed only for Jira MCP server via `uvx` |

---

## 🚀 Installation

### 1. Clone the repository

```bash
git clone https://github.com/vikkyvl/architecture_tests.git
cd architecture_tests
```

### 2. Install dependencies

```bash
go mod tidy
```

### 3. Build the binary

```bash
go build -o archguard ./cmd/archguard
```

The binary `archguard` will appear in the project root. You can move it to any directory in your `$PATH`.

---

## 🔑 Setup: API keys

```bash
cp .env.example .env
```

Open `.env` and uncomment the key for your chosen provider:

```env
# Choose ONE of the following:

# Anthropic Claude
ANTHROPIC_API_KEY=sk-ant-api03-...

# Google Gemini
GEMINI_API_KEY=AIza...

# OpenAI GPT
OPENAI_API_KEY=sk-...
```

> 💡 If multiple keys are present, the provider is auto-selected in priority order: **Anthropic → Gemini → OpenAI**.  
> Use `--provider` to force a specific one.

---

## ⚡ Quick Start

```bash
# Run against the bundled sample project (no API key tweaking needed for a demo)
./archguard analyze \
  -p ./testdata/sample-project \
  -r ./testdata/sample-project/archguard.yaml
```

This will:
1. Auto-detect the language (PHP)
2. Ask your consent before reading each source file
3. Produce `report.json` and `report.md` in the current directory

---

## 🖥️ Usage

ArchGuard has two commands: **`estimate`** (plan before spending tokens) and **`analyze`** (run the full analysis).

### `estimate` — plan before you run

```bash
./archguard estimate -p ./my-project -r archguard.yaml
```

`estimate` walks the file tree, counts source files, and calculates how many LLM tool calls the analysis will need — without making a single API call. It then asks whether to proceed:

- **Yes** → launches `analyze` immediately
- **No** → prints a ready-to-copy run plan with the exact commands to execute (including `--resume` chains for multi-run projects)

```
╭──────────────────────────────────────────────────────────────────────────────╮
│  Estimate                                                                    │
│                                                                              │
│  Project         dddsample-core (java)                                       │
│  Files           146                                                         │
│  Est. tool calls 308                                                         │
│  By layer        Domain          42 files  ~88 calls                        │
│                  Application     18 files  ~38 calls                        │
│                  Infrastructure  31 files  ~65 calls                        │
│  Runs needed     4  (use --resume between runs; --max-tool-calls=100 each)  │
╰──────────────────────────────────────────────────────────────────────────────╯
```

> The estimated call count scales with the number of **domain rules** in your config: each domain rule requires reading relevant file contents (not just imports), so more rules → more `read_file` calls → higher estimate.

### `analyze` — run the full analysis

```bash
./archguard analyze \
  --project  ./my-project \
  --rules    ./archguard.yaml \
  --provider anthropic
```

Press **Ctrl+C** at any time to interrupt cleanly — all subprocesses (external MCP servers) are shut down before exit.

### All flags (both commands)

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | `.` | Path to the project codebase to analyze |
| `--rules` | `-r` | — | Path to `archguard.yaml` **(required)** |
| `--docs` | `-d` | — | Path to a documentation file (passed to the LLM as extra context) |
| `--provider` | — | auto | LLM provider: `anthropic` / `gemini` / `openai` |
| `--api-key` | — | from `.env` | API key override (useful in CI without a `.env` file) |
| `--model` | — | provider default | Model override, e.g. `gpt-4o-mini`, `claude-haiku-4-5` |
| `--max-tool-calls` | — | `100` | Budget cap on total LLM tool calls; analysis stops when reached |
| `--timeout` | — | `10m` | Hard wall-clock timeout for the entire analysis run |
| `--interactive` | — | `true` | Show consent prompts; set `false` for CI to deny all unapproved calls |
| `--output-json` | `-j` | `report.json` | Path for the JSON report |
| `--output-md` | `-m` | `report.md` | Path for the Markdown report |
| `--resume` | — | — | `analyze` only: path to a **previous** `report.json` to continue from (see [Resume mode](#️-resume-mode)) |

### Examples

```bash
# Estimate first, then decide
./archguard estimate -p ./app -r archguard.yaml

# Use Gemini with a cheaper model override
./archguard analyze -p ./app -r archguard.yaml --provider gemini --model gemini-2.0-flash

# Use OpenAI, save reports to a custom path
./archguard analyze -p ./app -r archguard.yaml \
  --provider openai --model gpt-4o-mini \
  -j ./reports/analysis.json -m ./reports/analysis.md

# Non-interactive CI run (deny all unapproved reads)
./archguard analyze -p ./app -r archguard.yaml --interactive=false

# Continue a previous incomplete run
./archguard analyze -p ./app -r archguard.yaml --resume report.json -j report-full.json
```

---

## 📝 Configuration (`archguard.yaml`)

This file defines what to check and how. Place it anywhere and point `--rules` to it.

### Minimal example

```yaml
project:
  name: "my-app"
  language: php        # optional — auto-detected from file extensions if omitted
  src_root: src        # root used for namespace → path resolution

layers:
  - name: Domain
    namespaces: ["App\\Domain\\*"]
    paths: ["src/Domain/"]

  - name: Application
    namespaces: ["App\\Application\\*"]
    paths: ["src/Application/"]

  - name: Infrastructure
    namespaces: ["App\\Infrastructure\\*"]
    paths: ["src/Infrastructure/"]

rules:
  - from: Domain
    to: Application
    allow: false
    description: "Domain must not depend on Application"

  - from: Application
    to: Infrastructure
    allow: false
    description: "Application must not depend on Infrastructure"

  - from: Infrastructure
    to: Domain
    allow: true
    description: "Infrastructure implements domain interfaces"
```

### Full reference

```yaml
project:
  name: "payment-service"
  language: java          # php | typescript | python | java | csharp | go | javascript
  src_root: "src/main/java"
  exclude:                # glob patterns — matched files are hidden from the LLM
    - "src/test/**"
    - "target/**"

layers:
  - name: "Domain"
    namespaces: ["se.citerus.dddsample.domain.*"]   # used for dependency mapping
    paths: ["src/main/java/se/citerus/dddsample/domain/"]

rules:
  - from: "Domain"
    to: "Infrastructure"
    allow: false
    description: "Domain must not depend on Infrastructure"

domain_context:
  domain: "Logistics / Cargo Tracking"
  description: |
    Canonical DDD sample. Tracks ocean cargo shipments.
    The Cargo aggregate is the central root.
  domain_rules:
    - id: "domain_persistence_isolation"
      severity: critical                             # critical | high | medium | low
      description: "Domain classes must not import jakarta.persistence."
    - id: "value_object_immutability"
      severity: medium
      description: "Value objects must not expose public setters."

# Optional: external context sources queried BEFORE the agentic loop
external:
  - system: notion
    command: ["npx", "-y", "@notionhq/notion-mcp-server"]
    env:
      OPENAPI_MCP_HEADERS: '{"Authorization": "Bearer ${NOTION_API_TOKEN}", "Notion-Version": "2022-06-28"}'
    search_tool: "API-post-search"
    search_arg: "query"

# Optional runtime tuning (omit to use "auto" profile defaults)
runtime:
  profile: auto           # auto | conservative | aggressive
  max_file_bytes: 65536   # read_file soft cap; files larger than this are truncated
```

### Severity levels

| Level | When to use |
|-------|-------------|
| `critical` | Security issue, data corruption risk, must be fixed before shipping |
| `high` | Significant design violation that causes coupling or testability problems |
| `medium` | Suboptimal pattern, should be addressed in the next refactor |
| `low` | Style or convention issue, nice to fix when passing by |

---

## 🤖 LLM Providers

| Provider | Default model | Env variable | Notes |
|----------|--------------|--------------|-------|
| `anthropic` | `claude-opus-4-7` | `ANTHROPIC_API_KEY` | Best reasoning; prompt cache enabled by default |
| `gemini` | `gemini-2.5-flash` | `GEMINI_API_KEY` | Fast and cost-effective; large context window |
| `openai` | `gpt-4o` | `OPENAI_API_KEY` | Reliable; wide model selection |

### Rate limits & pacing

ArchGuard automatically paces requests to stay within each provider's quota:

| Provider | Default RPM | Min interval | Retry wait |
|----------|-------------|-------------|------------|
| Anthropic | 50 RPM | 1.2 s | 20 s |
| Gemini | 15 RPM (free tier) | 4 s | 30 s |
| OpenAI | 500 RPM | 0.12 s | 15 s |

> ⚠️ Gemini's free tier is 15 RPM. If you have a paid key, you can set a shorter `provider_retry_wait` in `runtime:`.

### Model overrides (cheaper / faster)

```bash
# Anthropic — cheaper haiku model
./archguard analyze -r archguard.yaml --provider anthropic --model claude-haiku-4-5

# OpenAI — cheaper mini model
./archguard analyze -r archguard.yaml --provider openai --model gpt-4o-mini

# Gemini — faster flash model
./archguard analyze -r archguard.yaml --provider gemini --model gemini-2.0-flash
```

---

## 🔐 Consent System

When the LLM wants to read a source file or query an external system, ArchGuard shows an interactive prompt:

```
╭──────────────────────────────────────────────────────────────────────────────╮
│ Consent required                                                             │
│                                                                              │
│ Tool         read_file                                                       │
│ path         src/Infrastructure/Persistence/PaymentRepository.php           │
│ Budget       87 calls remaining                                              │
│                                                                              │
│ > [a] Allow once     Only this call                                          │
│   [s] Allow session  All reads until analysis ends                          │
│   [p] Always allow   Save pattern to consent.yaml                           │
│   [d] Deny           Return denied result to the LLM                        │
╰──────────────────────────────────────────────────────────────────────────────╯
```

### Pre-approving patterns

Add rules to `.archguard/consent.yaml` (project-level) or `~/.config/archguard/consent.yaml` (global):

```yaml
allowed:
  - tool: read_file
    pattern: "glob:src/**"        # allow all reads under src/

  - tool: read_file
    pattern: "glob:src/Domain/**" # allow only Domain layer reads

  - tool: get_documentation
    pattern: all                  # always allow documentation reads
```

### CI / non-interactive mode

```bash
./archguard analyze -r archguard.yaml --interactive=false
```

With `--interactive=false`, any tool call not covered by a pre-approved consent rule is **automatically denied** (the LLM receives a "denied" result and continues without that file).

---

## ⏯️ Resume Mode

If an analysis run is interrupted (timeout, LLM error, or rate limit), all findings so far are saved to `report.json`.  
The next run can continue from where it left off:

```bash
# First run (got interrupted after 60 tool calls)
./archguard analyze -p ./my-app -r archguard.yaml -j report-part1.json

# Continue — skips already-analyzed files, pre-loads previous violations
./archguard analyze -p ./my-app -r archguard.yaml \
  --resume report-part1.json \
  -j report-full.json
```

### What `--resume` does

| | Behaviour |
|---|---|
| **Previous violations** | Pre-loaded into the new run; new ones are numbered v006, v007… |
| **Already-analyzed files** | Return a cached stub instead of re-reading; LLM is told not to re-analyze them |
| **Initial LLM context** | Lists remaining (unanalyzed) files and pre-loaded violations |
| **Final report** | `analyzed_modules` = merged from both runs; `skipped_modules` recomputed |

> 💡 Resume works even if the previous run was **complete** — useful for adding new rules and re-checking only untouched files.

---

## ⚙️ Runtime Profiles

Control how aggressively ArchGuard uses the LLM quota. Add to `archguard.yaml`:

```yaml
runtime:
  profile: auto   # auto | conservative | aggressive
```

| Profile | Max retries | Retry wait | Context pruning | Best for |
|---------|------------|------------|-----------------|----------|
| `auto` *(default)* | 5 | 20 s | After 10 turns | Most use cases |
| `conservative` | 7 | 30 s | After 6 turns | Free-tier keys, flaky connections |
| `aggressive` | 3 | 10 s | After 10 turns | Paid keys with high quotas |

Individual fields can override a profile preset:

```yaml
runtime:
  profile: auto
  max_file_bytes: 32768          # override just the file size cap
  provider_max_retries: 7        # override just the retry count
  respect_rate_limit_headers: true  # preemptive sleep on near-empty quota headers
```

---

## 🌍 External Integrations

ArchGuard can query Notion, Jira, or YouTrack for domain context **before** starting the analysis. This lets the LLM understand business requirements without exposing your code to external services.

### How it works

1. The tool `get_external_context` is **filtered out of the LLM-visible catalog**
2. ArchGuard queries configured systems **deterministically** before the agentic loop
3. Retrieved context is prepended to the initial LLM message as plain text

### Notion

```bash
# Install the MCP server
npm install -g @notionhq/notion-mcp-server
```

Add to `.env`:
```env
NOTION_API_TOKEN=ntn_xxxxxxxxxxxxxxxxx
```

Add to `archguard.yaml`:
```yaml
external:
  - system: notion
    command: ["npx", "-y", "@notionhq/notion-mcp-server"]
    env:
      OPENAPI_MCP_HEADERS: '{"Authorization": "Bearer ${NOTION_API_TOKEN}", "Notion-Version": "2022-06-28"}'
    search_tool: "API-post-search"
    search_arg: "query"
```

> ⚠️ Notion pages must be explicitly shared with the integration (Share → Invite → your integration).

### Jira

```bash
pip install uv   # or: brew install uv
```

Add to `.env`:
```env
JIRA_URL=https://your-org.atlassian.net
JIRA_USERNAME=you@example.com
JIRA_API_TOKEN=ATATT3xFfGF0...
JIRA_PROJECT_KEY=PROJ,OPS
```

Add to `archguard.yaml`:
```yaml
external:
  - system: jira
    command: ["uvx", "mcp-atlassian"]
    env:
      JIRA_URL: "${JIRA_URL}"
      JIRA_USERNAME: "${JIRA_USERNAME}"
      JIRA_API_TOKEN: "${JIRA_API_TOKEN}"
      JIRA_PROJECTS_FILTER: "${JIRA_PROJECT_KEY}"
    search_tool: "jira_search"
    search_arg: "jql"
    query_template: 'text ~ "%s"'
    default_query: "architecture payment audit"
```

### YouTrack

Add to `.env`:
```env
YOUTRACK_URL=https://your-org.youtrack.cloud
YOUTRACK_TOKEN=perm:base64token
```

Add to `archguard.yaml`:
```yaml
external:
  - system: youtrack
    command: ["npx", "-y", "@vitalyostanin/youtrack-mcp"]
    env:
      YOUTRACK_URL: "${YOUTRACK_URL}"
      YOUTRACK_TOKEN: "${YOUTRACK_TOKEN}"
    search_tool: "issues_search"
    search_arg: "query"
    default_query: "architecture payment"
```

---

## 📊 Reports

After analysis, two files are written (default: `report.json` and `report.md`).

### `report.json` — machine-readable

```json
{
  "project_name": "payment-service",
  "language": "php",
  "llm_provider": "anthropic",
  "llm_model": "claude-opus-4-7",
  "analyzed_at": "2026-05-26T18:12:48Z",
  "duration": "2m40s",
  "files_scanned": 12,
  "tool_calls": 34,
  "incomplete": false,
  "violations": [
    {
      "id": "v001",
      "file": "src/Domain/Model/Order.php",
      "line": 5,
      "severity": "critical",
      "category": "structural",
      "rule": "domain_persistence_isolation",
      "description": "Domain class imports Doctrine ORM — persistence concern in domain layer.",
      "suggestion": "Move persistence annotations to Infrastructure; keep Domain as plain objects."
    }
  ],
  "metrics": {
    "total_violations": 1,
    "by_severity": { "critical": 1 },
    "cache_hit_ratio": 0.45
  },
  "analyzed_modules": ["src/Domain/Model/Order.php"],
  "skipped_modules": [],
  "audit_log": [...]
}
```

### `report.md` — human-readable

```markdown
# Architecture Analysis Report: payment-service

- Language: php
- LLM: anthropic (claude-opus-4-7)
- Duration: 2m40s  |  Files scanned: 12  |  Tool calls: 34

## By Severity
| Severity | Count |
|----------|-------|
| critical | 1     |

## Violations

### CRITICAL

**v001 · [structural] domain_persistence_isolation**
- **File:** `src/Domain/Model/Order.php` (line 5)
- **Description:** Domain class imports Doctrine ORM — persistence concern in domain layer.
- **Suggestion:** Move persistence annotations to Infrastructure; keep Domain as plain objects.
```

### Useful fields for CI integration

| Field | Type | Description |
|-------|------|-------------|
| `incomplete` | bool | `true` if the run hit `--max-tool-calls` or `--timeout` |
| `metrics.total_violations` | int | Use this as a CI gate |
| `skipped_modules` | []string | Files not analyzed in this run (feed to `--resume` next time) |
| `metrics.cache_hit_ratio` | float | Anthropic/OpenAI prompt cache efficiency (0–1) |

---

## 🧪 Development

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run against the bundled DDD sample (Java)
./archguard analyze \
  -p ./testdata/dddsample-core \
  -r ./testdata/dddsample-core/archguard.yaml \
  --provider anthropic

# Lint & vet
go vet ./...
```
