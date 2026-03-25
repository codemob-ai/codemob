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

Commands:
```
codemob new [name]          # create mob + launch agent
codemob resume [name]       # resume a mob (continue previous session)
codemob open [name]         # open a mob (fresh agent session)
codemob list                # list all mobs
codemob path [name]         # print worktree path (interactive if no name)
codemob cd <name>           # cd into a mob's worktree (shell function)
codemob cd root             # cd back to repo root
codemob init                # first-time setup (global + repo)
codemob reinit              # alias for init (idempotent)
codemob remove <name>       # remove a mob (accepts name or index)
codemob purge               # remove all mobs (with confirmation)
codemob info                # show diagnostic information
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
claude --mob [name]         # → codemob new --agent claude [name]
claude --new-mob [name]     # same as --mob
claude --resume-mob <name>  # → codemob resume <name>
claude --open-mob <name>    # → codemob open --agent claude <name>
codex --mob [name]          # → codemob new --agent codex [name]
codex --resume-mob <name>   # → codemob resume <name>
codex --open-mob <name>     # → codemob open --agent codex <name>
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
KNOWN_ISSUES.md         # tracked issues not yet fixed
```

## Data model

`.codemob/config.json`:
```json
{
  "default_agent": "claude",
  "base_branch": "main",
  "repo_root": "/Users/you/repos/android",
  "mobs_dir": "/Users/you/repos/.codemob/android/mobs",
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

**Config stores explicit absolute paths.** Both `repo_root` and `mobs_dir` are always set to absolute paths during init. If reality diverges (repo moved, mobs dir deleted), codemob fails with a hard error telling the user to reinit. This is intentional - we accept that repo moves require reinit rather than adding dynamic resolution or fallback logic.

`.codemob/queue.json` (transient, written by slash commands):
```json
{
  "action": "switch",
  "target": "other-mob",
  "mob": ""
}
```

## Design philosophy

codemob is early-stage. Optimize for the common user, not power users. Consider what power users want/need, but don't add complexity to accommodate edge cases they create (e.g., hand-editing config files). Keep the product simple and predictable - complexity is the enemy at this stage.

## CLI conventions

**Flag rejection:** Every command must explicitly reject unknown `--` flags with a clear error. Never silently treat a flag-like arg as a positional value (e.g., `--typo` should not become a mob name). Each command's arg-parsing loop has its own `strings.HasPrefix(arg, "--")` check in the default branch.

**Interactive picker:** When a command needs the user to select a mob, use the shared `pickMob()` function in `root.go`. Configure it via `pickerOpts` (marker, default value, root hint, output writer) rather than duplicating the table/prompt logic.

**Bug fixes need tests:** When fixing a bug - whether reported by the user or discovered independently - always consider adding a regression test in `integration_test.go`. The test suite is comprehensive and easy to extend. A bug that was worth fixing is worth preventing from coming back.

## Slash commands

Slash command `.md` files in `.claude/commands/` are generated by `codemob init` from definitions in `internal/mob/init.go`. The `.md` files on disk are snapshots - not the source of truth. When modifying slash command prompts, always edit the `slashCommandDefs` map in `init.go`, not the generated `.md` files.

## Core/shell interface

The Go binary is the primary interface — it handles everything including agent launching (as child processes with a trampoline loop). The shell script is an optional enhancement that provides `mob` alias and `claude --new-mob` / `codex --new-mob` wrappers, plus post-exit queue checking.

## Session tracking (CODEMOB_SESSION)

`codemob-shell.sh` sets `$CODEMOB_SESSION` (a UUID) once per terminal window at shell startup. The Go binary uses this as a file key under `.codemob/sessions/<uuid>` to track the last active mob per terminal.

This enables `codemob resume` (no name) to default to the last-used mob in that terminal — even with parallel sessions in different terminals.

**How it works:**
- `writeLastMob()` in `launchAgent` writes the mob name on normal exit
- `readLastMob()` in `cmdResume` reads it to pre-select the default
- On remove/drop (empty workdir), nothing is written — stale entry is harmless since the mob won't exist in config anymore

**Edge cases to keep in mind when modifying queue/trampoline logic:**
- Any new queue action that removes a mob must NOT call `writeLastMob` (same as "remove" today)
- Any new queue action that switches to a different mob must update `workdir` before the loop continues (same as "switch"/"new" today)
- The session file is never deleted — orphaned files from closed terminals are harmless (mob existence is validated on read)
