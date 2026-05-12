# ArchGuard Analyzer

> AI-powered architectural analyzer for layered applications.  
> Finds violations in layer dependencies, domain rules, and design principles.  
> **Source code never leaves your machine** — the LLM works through controlled tools, not raw file uploads.

---

## Features

- Checks **layer dependency rules** — e.g. Infrastructure must not depend on Presentation
- Checks **domain-specific rules** — custom rules defined in YAML
- Supports **PHP, TypeScript, Python, Java, C#, Go, JavaScript** (auto-detected)
- Works with **Anthropic Claude, Google Gemini, OpenAI GPT**
- **Consent system** — interactive prompts before reading any file; session approvals are temporary
- **Audit log** — every tool call recorded in the report
- Outputs **JSON + Markdown** reports

---

## Installation

```bash
git clone https://github.com/vikkyvl/architecture_tests.git
cd architecture_tests
```

> Requires **Go 1.24+** — check with `go version`.

---

## Setup

**1. Build**

```bash
go mod tidy
go build -o archguard ./cmd/archguard
```

**2. Add your API key**

```bash
cp .env.example .env
```

Then open `.env` and uncomment the key for your provider:

```env
# Pick one LLM provider:
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AIza-...
OPENAI_API_KEY=sk-...

# Optional — enables get_external_context tool:
NOTION_API_KEY=secret_...
```

---

## Usage

```bash
./archguard analyze \
  --project  ./my-project \
  --rules    ./archguard.yaml \
  --provider anthropic          # or gemini / openai
```

**All flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | `.` | Path to the project codebase |
| `--rules` | `-r` | — | Path to `archguard.yaml` **(required)** |
| `--docs` | `-d` | — | Path to project documentation file |
| `--provider` | — | auto | LLM provider: `anthropic` / `gemini` / `openai` |
| `--api-key` | — | from `.env` | API key override |
| `--model` | — | provider default | Model override (e.g. `gpt-4o-mini`) |
| `--max-tool-calls` | — | `100` | Budget cap on LLM tool calls |
| `--timeout` | — | `10m` | Hard timeout for the full analysis |
| `--interactive` | — | `true` | Prompt for consent on sensitive tool calls |
| `--output-json` | `-j` | `report.json` | JSON report path |
| `--output-md` | `-m` | `report.md` | Markdown report path |

**Quick run with the sample project:**

```bash
./archguard analyze \
  -p ./testdata/sample-project \
  -r ./testdata/sample-project/archguard.yaml
```

---

## Configuration (`archguard.yaml`)

```yaml
project:
  name: MyPaymentApp
  language: php        # optional — auto-detected if omitted
  src_root: src        # root for namespace-to-path resolution

layers:
  - name: Domain
    namespaces: ["App\\Domain\\*"]
    paths: ["src/Domain"]

  - name: Application
    namespaces: ["App\\Application\\*"]
    paths: ["src/Application"]

  - name: Infrastructure
    namespaces: ["App\\Infrastructure\\*"]
    paths: ["src/Infrastructure"]

rules:
  - from: Infrastructure
    to: Domain
    allow: true
  - from: Application
    to: Domain
    allow: true
  - from: Application
    to: Infrastructure
    allow: false
    description: "Application must not depend on Infrastructure"

domain_context:
  domain: Payments
  description: "Online payment processing system"
  rules:
    - id: DR-01
      severity: high
      description: "PaymentService must not directly access PaymentRepository"
    - id: DR-02
      severity: medium
      description: "All money amounts must use a value object, not raw float"
```

**Severity levels:** `critical` · `high` · `medium` · `low`

**Violation categories:** `layer_dependency` · `domain_rule` · `solid` · `grasp`

---

## LLM Providers

| Provider | Default model | Env variable |
|----------|--------------|--------------|
| `anthropic` | `claude-opus-4-1-20250805` | `ANTHROPIC_API_KEY` |
| `gemini` | `gemini-2.5-flash` | `GEMINI_API_KEY` |
| `openai` | `gpt-4o` | `OPENAI_API_KEY` |

If `--provider` is not set, the first key found in `.env` is used (Anthropic → Gemini → OpenAI).

Override the model for a cheaper/faster run:

```bash
./archguard analyze -r archguard.yaml --provider openai --model gpt-4o-mini
./archguard analyze -r archguard.yaml --provider gemini --model gemini-2.5-flash
```

---

## Consent system

When the LLM wants to read a file or fetch external context, ArchGuard asks for your permission:

```
╭──────────────────────────────────────────────────────────────────────────────╮
│ Consent required                                                             │
│                                                                              │
│ Tool         read_file                                                       │
│ path         src/Infrastructure/Persistence/PaymentRepository.php           │
│ Budget       87 calls remaining                                              │
│                                                                              │
│ > [a] Allow once     Only this tool call                                     │
│   [s] Allow session  Same tool until review ends                             │
│   [p] Always allow   Remember this pattern                                   │
│   [d] Deny           Return denied tool result                               │
╰──────────────────────────────────────────────────────────────────────────────╯
```

Pre-approved rules for non-interactive runs can be placed in `.archguard/consent.yaml`
(project-level) or `~/.config/archguard/consent.yaml` (global):

```yaml
allowed:
  - tool: read_file
    pattern: "glob:src/**"
  - tool: get_documentation
    pattern: all
```

Run with `--interactive=false` in CI to deny all non-pre-approved calls automatically.

---

## Reports

After analysis, two files are written:

**`report.json`** — full machine-readable result with violations, metrics, audit log.

**`report.md`** — human-readable summary:

```
# Architecture Analysis Report: MyPaymentApp

- Language: php
- LLM: anthropic (claude-opus-4-1-20250805)
- Analyzed at: 2026-05-12 14:32:01
- Duration: 43s
- Files scanned: 4
- Tool calls: 18
- Violations: 2

## By Severity
| Severity | Count |
|----------|-------|
| high     | 1     |
| medium   | 1     |

## Violations

### HIGH
**1. [layer_dependency] Infrastructure → Application dependency**
- File: `src/Infrastructure/Persistence/PaymentRepository.php` (line 8)
- Description: ...
- Suggestion: ...
```
