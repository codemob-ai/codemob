# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is codemob

codemob is a CLI tool (Go) that manages git worktrees as isolated AI agent workspaces called "mobs." It abstracts away manual worktree management so you can spin up, list, switch between, and clean up workspaces without touching git commands directly.

Key ideas:
- No UI beyond terminal — CLI tool only
- Agent-agnostic: works transparently with Claude Code initially, other terminal-based AI agents later
- codemob owns the worktree lifecycle: create, list, resume, remove
- Git is the source of truth — codemob metadata is supplementary. On every operation, reconcile against `git worktree list`. If a worktree exists in metadata but not in git, clean it up silently. If it exists in git but not in metadata, it's not managed by codemob.

## CLI interfaces

Two ways to interact:
- `mob` command — direct management (`mob new`, `mob ls`, `mob resume <name>`, `mob rm <name>`)
- Agent wrapper — e.g. `claude --new-mob` / `claude --resume-mob <name>` via shell alias/wrapper that intercepts `--*-mob` flags, delegates to `mob`, and passes everything else through to the real agent binary

## Directory structure

All codemob state lives inside the repo it's initialized in:

```
.codemob/
  config.json       # repo-level codemob config
  mobs/
    fix-auth-bug/   # actual worktree
    add-caching/    # actual worktree
```

`.codemob/` must be in `.gitignore`.

## Tech stack

- Language: Go
- CLI framework: Cobra
- Distribution: GoReleaser + Homebrew tap
