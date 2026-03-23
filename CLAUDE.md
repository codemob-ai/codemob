# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Brand colors

- `#002900` — background (dark green)
- `#e7dc60` — accent (yellow-gold)

## What is codemob

codemob is a CLI tool that manages git worktrees as isolated AI agent workspaces called "mobs." It abstracts away manual worktree management so you can spin up, list, switch between, and clean up workspaces without touching git commands directly.

Key ideas:
- No UI beyond terminal — CLI tool only
- Agent-agnostic: works transparently with Claude Code initially, other terminal-based AI agents later
- codemob owns the worktree lifecycle: create, list, resume, remove
- Git is the source of truth — codemob metadata is supplementary. On every operation, reconcile against what exists on disk. If a worktree exists in metadata but not on disk, clean it up silently.

## Architecture

Two layers:
- **`codemob`** (Go binary) — all logic: config management, git operations, reconciliation, JSON. Launches agents as child processes (`exec.Command` with `cmd.Dir`), implements a trampoline loop that checks `queue.json` after agent exit for seamless switching.
- **`codemob-shell.sh`** (bash) — sourced into shell via `.zshrc`. Defines `mob` alias and `claude`/`codex` wrappers that intercept `--*-mob`/`--*-codemob` flags. Also checks `queue.json` after agent exit for the shell-launched path. Preserves agent exit codes.

## CLI interface

Core workflow — flags on `codemob`/`mob`:
```
codemob --new [name]        # create mob + launch agent
codemob --resume <name>     # resume a mob (launch agent in worktree)
codemob --switch <name>     # alias for --resume
codemob --list              # list all mobs
codemob --list-others       # list mobs excluding current (for slash commands)
```

Management — subcommands:
```
codemob init                # first-time setup (global + repo)
codemob reinit              # alias for init (idempotent)
codemob remove <name>       # remove a mob (accepts name or index)
codemob clear               # remove all mobs (with confirmation)
codemob uninstall           # remove all codemob setup (global + local)
```

Options:
```
--no-launch                 # skip launching the agent
--agent <name>              # override agent (claude, codex)
--force                     # force remove
--version                   # show version
```

Claude/Codex wrappers (installed by init):
```
claude --new-mob [name]     # → codemob --new --agent claude [name]
claude --resume-mob <name>  # → codemob --resume <name>
codex --new-mob [name]      # → codemob --new --agent codex [name]
```

Internal (used by slash commands):
```
codemob queue <action> [target]   # write queued action for trampoline
codemob --check-queue             # process queued action (called by shell wrapper)
```

## Build

```bash
make build          # build the binary
make install        # dev install (emulates Homebrew layout at /opt/homebrew)
make test           # run all tests
make clean          # remove build artifacts
```

## Project structure

```
codemob-shell.sh        # optional bash aliases (sourced into .zshrc)
main.go                 # Go entry point
cmd/
  root.go               # command dispatch, all commands, agent launching, trampoline
internal/
  git/git.go            # git command wrappers
  mob/mob.go            # data model, config, reconciliation, name validation
  mob/init.go           # init/uninstall, slash commands, Codex prompts, Claude permissions
  mob/next.go           # queue.json read/write/clear
  mob/names.go          # random name generation (adjective-fruit)
  mob/integration_test.go
Makefile                # build/install/test
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

`.codemob/queue.json` (transient, written by slash commands):
```json
{
  "action": "switch",
  "target": "other-mob",
  "mob": ""
}
```

## Core/shell interface

The Go binary is the primary interface — it handles everything including agent launching (as child processes with a trampoline loop). The shell script is an optional enhancement that provides `mob` alias and `claude --new-mob` / `codex --new-mob` wrappers, plus post-exit queue checking.
