# ArchGuard Analyzer

Architectural analyzer tool for layered applications, powered by LLM.
Source code never leaves the user's machine.

## Setup

```bash
go mod tidy
go build -o project ./cmd/archguard
cp .env.example .env   # add your API key
```

## Usage

```bash
./project analyze -p ./testdata/sample-project -r ./testdata/sample-project/archguard.yaml
```

## Structure

```
cmd/archguard/        CLI entry point
client/
  cli/                Orchestrator (init + review flows)
  detector/           Code Language Detector
  mcp/                MCP Resolver (tool execution)
  consent/            Consent Manager
  audit/              Audit Log
api/
  context/            Context Resolver (prompt builder)
  llm/                LLM Resolver (Anthropic, Gemini)
  result/             Result Reviewer (dedup, render)
shared/
  config/             YAML config + .env loader
  models/             Violation, AnalysisResult, AuditEntry
```
