# 🏛️ ArchGuard Analyzer

![Version](https://img.shields.io/badge/version-0.1.0-blue)
![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)

> **AI-powered architectural analyzer for layered applications.**  
> Finds violations in layer dependencies, domain rules, and design principles — using your LLM as the reasoning engine.  
> 🔒 **Source code never leaves your machine** — the LLM reads only what it explicitly asks for, through a controlled consent layer.

---

## ✨ Features

- 🗂️ **Layer dependency checks** — Domain must not depend on Infrastructure, etc.
- 📜 **Domain-specific rules** — custom semantic rules defined in plain YAML
- 🌐 **Multi-language** — PHP, TypeScript, JavaScript, Python, Java, C#, Go, Ruby, Kotlin, Swift, Rust, C, C++, Scala, Lua, Bash, Elixir, HCL, SQL (auto-detected)
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
| **Node.js** *(optional)* | 18+ | `node --version` — needed only for Notion / YouTrack MCP servers |
| **Python + uv** *(optional)* | Python 3.9+, uv any | `uv --version` — needed only for Jira MCP server via `uvx` |

### Install Go

**macOS (Homebrew):**
```bash
brew install go
```

**Linux:**
```bash
wget https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**Windows:** download the installer from [go.dev/dl](https://go.dev/dl/).

Verify: `go version` → should print `go1.24` or higher.

### Install C compiler (required for tree-sitter)

**macOS:** ships with Apple Clang via Xcode Command Line Tools:
```bash
xcode-select --install
```

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install -y build-essential
```

**Linux (Fedora/RHEL):**
```bash
sudo dnf install -y gcc
```

**Windows:** install [MSYS2](https://www.msys2.org/) and run `pacman -S mingw-w64-ucrt-x86_64-gcc`.

Verify: `clang --version` or `gcc --version`.

### Install Node.js *(optional — Notion / YouTrack only)*

**macOS:**
```bash
brew install node
```

**Linux / Windows:** download the LTS installer from [nodejs.org](https://nodejs.org/).

Verify: `node --version` → should print `v18` or higher.

### Install Python + uv *(optional — Jira only)*

**macOS / Linux:**
```bash
brew install uv          # macOS
# or
curl -LsSf https://astral.sh/uv/install.sh | sh   # Linux / macOS
```

**Windows:**
```powershell
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

Verify: `uv --version`.

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
| `--resume` | — | — | `analyze` only: path to a **previous** `report.json` to continue from |

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

Every field the config parser understands is listed below. Only `project.name`, at least one `layers` entry, and the `rules` list are practically required — everything else is optional and falls back to the documented default.

```yaml
# ───────────────────────────── project ─────────────────────────────
project:
  name: "payment-service"        # required — shown in the report header
  language: java                 # optional — auto-detected from extensions if omitted.
                                 #   php | typescript | javascript | python | java | csharp |
                                 #   go | ruby | kotlin | swift | rust | c | cpp | scala |
                                 #   lua | bash | elixir | hcl | sql
  src_root: "src/main/java"      # root used for namespace → file-path resolution
  exclude:                       # glob patterns — matched files are hidden from the LLM entirely
    - "src/test/**"
    - "target/**"

# ───────────────────────────── layers ──────────────────────────────
layers:                          # logical layers; declaration order is irrelevant
  - name: "Domain"               # referenced by rules.from / rules.to and by *.applies_to
    namespaces:                  # import prefixes mapped to this layer (dependency analysis)
      - "se.citerus.dddsample.domain.*"
    paths:                       # filesystem paths mapped to this layer
      - "src/main/java/se/citerus/dddsample/domain/"

# ───────────────────────────── rules ───────────────────────────────
rules:                           # allowed / forbidden dependencies between layers
  - from: "Domain"               # source layer name (must match a layers[].name)
    to: "Infrastructure"         # target layer name
    allow: false                 # false = forbidden (a violation), true = explicitly allowed
    description: "Domain must not depend on Infrastructure"

# ────────────────────────── domain_context ─────────────────────────
domain_context:                  # optional — semantic domain knowledge for the LLM
  domain: "Logistics / Cargo Tracking"
  description: |                 # free-form text prepended to the system prompt
    Canonical DDD sample. Tracks ocean cargo shipments.
  domain_rules:                  # semantic rules verified by reading file *contents*
    - id: "domain_persistence_isolation"   # stable id, used as the violation's rule key
      severity: critical                   # critical | high | medium | low
      description: "Domain classes must not import jakarta.persistence."

# ───────────────────────── design_principles ───────────────────────
design_principles:               # optional — generic design principles (SOLID, DRY, …)
  - id: "srp"
    severity: high               # critical | high | medium | low
    description: "Each class must have a single responsibility."
    applies_to:                  # optional — restrict the principle to specific layers
      - "Application"

# ───────────────────────────── architecture ────────────────────────
architecture:                    # optional — high-level style + cross-cutting invariants
  style: "Hexagonal (Ports & Adapters)"
  description: "Application depends only on Domain ports."   # optional
  invariants:                    # optional
    - id: "ports_only"
      severity: critical         # critical | high | medium | low
      description: "Application must depend on interfaces, not concrete adapters."
      applies_to:                # optional — restrict the invariant to specific layers
        - "Application"

# ───────────────────────────── external ────────────────────────────
external:                        # optional — domain context queried BEFORE the agentic loop
  - system: jira                 # label only: notion | jira | youtrack
    command: ["uvx", "mcp-atlassian"]      # subprocess that speaks MCP over stdin/stdout
    env:                         # env for the subprocess; ${VAR} is expanded from .env
      JIRA_URL: "${JIRA_URL}"
    search_tool: "jira_search"   # MCP tool name to invoke
    search_arg: "jql"            # name of the argument that carries the query string
    query_template: 'text ~ "%s"'         # optional — %s is replaced by the derived query
    project_filter: "PROJ,OPS"            # optional — restrict results to these projects
    default_query: "architecture payment audit"   # optional — used when no query is derived
    include: ["*.md"]            # optional — glob allow-list for returned documents
    exclude: ["archive/**"]      # optional — glob deny-list for returned documents

# ───────────────────────────── runtime ─────────────────────────────
runtime:                         # optional — quota / pacing tuning (omit to use "auto")
  profile: auto                  # auto | conservative | aggressive (sets the defaults below)
  max_file_bytes: 65536          # read_file soft cap (default 65536); larger files truncated
  prune_after_turns: 10          # begin pruning old tool results after N LLM turns
  keep_recent_turns: 4           # always keep the last N turns un-pruned
  provider_max_retries: 5        # retries on transient LLM errors
  provider_retry_wait: 20s       # base wait between retries (Go duration: 500ms, 30s, 2m…)
  anthropic_cache_tail: true     # place an Anthropic prompt-cache breakpoint at the tail
  respect_rate_limit_headers: true  # preemptively sleep when quota headers are near-empty
```

> Any individual `runtime` field you set **overrides just that field** of the chosen `profile`; everything you leave out keeps the profile's preset value. See the **Runtime Profiles** section below for the per-profile presets.

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

### Available models

**Anthropic** ([full list](https://docs.anthropic.com/en/docs/about-claude/models))

| Model |
|-------|
| `claude-opus-4-7` *(default)* |
| `claude-opus-4-5` |
| `claude-sonnet-4-5` |
| `claude-haiku-4-5` |

**Google Gemini** ([full list](https://ai.google.dev/gemini-api/docs/models))

| Model |
|-------|
| `gemini-2.5-flash` *(default)* |
| `gemini-2.5-pro` |
| `gemini-2.0-flash` |

**OpenAI** ([full list](https://platform.openai.com/docs/models))

| Model |
|-------|
| `gpt-4o` *(default)* |
| `gpt-4o-mini` |
| `o3` |
| `o4-mini` |

> ⚠️ OpenAI reasoning models (`o1`, `o3`, `o4-mini`, `gpt-5`…) use `max_completion_tokens` instead of `max_tokens` — ArchGuard detects this automatically.

### Model override examples

```bash
./archguard analyze -r archguard.yaml --provider anthropic --model claude-haiku-4-5
./archguard analyze -r archguard.yaml --provider openai    --model gpt-4o-mini
./archguard analyze -r archguard.yaml --provider gemini    --model gemini-2.5-pro
```

---

## 🔐 Consent System

When the LLM wants to read a source file or query an external system, ArchGuard shows an interactive prompt with four options: **Allow once**, **Allow session**, **Always allow** (saves to `consent.yaml`), or **Deny**.

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
      "category": "layer_dependency",
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
  "audit_log": []
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

### View the report in your browser

A small local viewer renders `report.md` as a styled dashboard — severity badges, colored violation cards, audit log, and the analyzed-module checklist — and **live-reloads** whenever you re-run the analysis.

```bash
# Renders report.md at http://localhost:8765 and opens your browser
node tools/mdserve/server.js
```

| Flag | Default | Description |
|------|---------|-------------|
| `-f`, `--file` | `report.md` | Markdown file to render |
| `-p`, `--port` | `8765` | Port to listen on |
| `--no-open` | — | Do not auto-open the browser |

```bash
node tools/mdserve/server.js -f reports/analysis.md -p 9000
```

The viewer runs entirely offline on `localhost` — nothing is sent anywhere. Stop it with **Ctrl+C** (or `kill $(lsof -ti tcp:8765)` if it runs in the background). Requires Node.js 18+.

