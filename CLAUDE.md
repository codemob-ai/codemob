# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is codemob

codemob is a CLI tool that manages git worktrees as isolated AI agent workspaces called "mobs." It abstracts away manual worktree management so you can spin up, list, switch between, and clean up workspaces without touching git commands directly.

Key ideas:
- No UI beyond terminal — CLI tool only
- Agent-agnostic: works transparently with Claude Code initially, other terminal-based AI agents later
- codemob owns the worktree lifecycle: create, list, resume, remove
- Git is the source of truth — codemob metadata is supplementary. On every operation, reconcile against what exists on disk. If a worktree exists in metadata but not on disk, clean it up silently.

## Architecture

Two layers:
- **`codemob.sh`** (bash) — sourced into shell via `.zshrc`. Defines `codemob`/`mob`/`claude` functions. Thin layer: parses flags, calls `codemob-core`, handles `cd` + agent launch.
- **`codemob-core`** (Go binary) — all logic: config management, git operations, reconciliation, JSON. Outputs `KEY=VALUE` lines for the bash layer to consume.
- **`init.sh`** (bash) — standalone setup script for global + repo initialization.

## CLI interface

Core workflow — flags on `codemob`/`mob`:
```
codemob --new [name]        # create mob + launch agent
codemob --resume <name>     # resume a mob (cd + launch agent)
codemob --switch <name>     # alias for --resume
codemob --list              # list all mobs
```

Management — subcommands:
```
codemob init                # first-time setup (global + repo)
codemob reinit              # alias for init (idempotent)
codemob remove <name>       # remove a mob
```

Claude wrapper (installed by init):
```
claude --new-mob [name]     # → codemob --new [name]
claude --resume-mob <name>  # → codemob --resume <name>
```

## Build

```bash
go build -o codemob-core .   # build the Go brain
```

No build step needed for `codemob.sh` or `init.sh` — they're plain bash.

## Project structure

```
codemob.sh              # bash shell functions (sourced into .zshrc)
codemob-core            # Go binary (build artifact, gitignored)
init.sh                 # standalone setup script
main.go                 # Go entry point
cmd/
  root.go               # command dispatch + all core commands
internal/
  git/git.go            # git command wrappers
  mob/mob.go            # data model, config, reconciliation
claude-commands/        # Claude Code slash commands (/mob-ls, /mob-new, etc.)
```

## Data model

`.codemob/config.json`:
```json
{
  "default_agent": "claude",
  "base_branch": "main",
  "mobs": [
    {
      "name": "fix-auth-bug",
      "branch": "mob/fix-auth-bug",
      "created_at": "2026-03-22T14:00:00Z",
      "agent": "claude"
    }
  ]
}
```

## Core/shell interface

The Go binary outputs `CODEMOB_KEY=value` lines. The bash layer parses these to get paths, agent names, etc., then handles shell-level operations (cd, launching agents). This separation means the Go binary never touches the shell and is replaceable.
