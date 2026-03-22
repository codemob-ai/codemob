![codemob](img/banner.png)

Git worktree manager for AI agent workspaces. Run multiple AI coding sessions in isolated worktrees without touching git commands.

## What it does

Each "mob" is a git worktree where an AI agent (Claude Code, Codex, etc.) works independently. codemob manages the lifecycle: create, list, switch, remove.

```
codemob --new brave-mango       # creates worktree, launches claude in it
codemob --list               # shows all mobs
codemob --resume brave-mango    # cd into worktree, launch agent
codemob remove brave-mango      # clean up worktree + branch
```

## Install

```bash
# build from source
git clone https://github.com/codemob-ai/codemob.git
cd codemob
go build -o codemob .

# put the binary on your PATH, then:
codemob init
```

`codemob init` sets up:
- Shell integration (`mob` alias, `claude --new-mob` wrapper)
- Global gitignore for `.codemob/` directories
- Claude Code slash commands (`/mob-list`, `/mob-switch`, etc.)
- Claude Code permissions for codemob commands

## Usage

### From terminal

```bash
codemob --new                    # auto-generated name (e.g., wild-kumquat)
codemob --new brave-mango           # named mob
codemob --new --agent codex      # use codex instead of claude
codemob --list                   # list all mobs with indices
codemob --resume brave-mango        # resume by name
codemob --resume 2               # resume by index
codemob remove brave-mango          # remove a mob
codemob clear                    # remove all mobs
```

### Shell aliases (after `codemob init`)

```bash
mob --new brave-mango               # short alias
claude --new-mob brave-mango        # create mob + launch claude
claude --resume-mob brave-mango     # resume mob in claude
codex --new-codemob brave-mango     # create mob + launch codex
```

### From inside Claude Code

Slash commands installed by `codemob init`:

- `/mob-list` — list all mobs
- `/mob-new` — create a new mob (queued, launches after exit)
- `/mob-switch` — switch to another mob (queued, launches after exit)
- `/mob-switch-agent` — swap agent mid-session (e.g., claude to codex)
- `/mob-remove` — remove a mob

All commands also work with `codemob-` prefix (`/codemob-list`, etc.).

## How switching works

codemob uses a trampoline pattern. When you `/mob-switch` from inside Claude:

1. The slash command writes a `queue.json` file
2. You exit Claude
3. codemob (which launched Claude as a child process) reads the queue
4. codemob launches the target mob's agent automatically

This also works when Claude is launched directly via the shell wrapper — the `claude()` function checks for queued actions after Claude exits.

## Architecture

- **`codemob`** (Go binary) — all logic. Config, git ops, reconciliation, agent launching via `os.Chdir` + child process.
- **`codemob-shell.sh`** (bash, optional) — sourced into `.zshrc`. Just aliases: `mob`, `claude --new-mob`, `codex --new-mob`.

All state lives in `.codemob/` inside the repo:

```
.codemob/
  config.json       # mobs metadata, default agent, base branch
  queue.json        # pending action (written by slash commands)
  mobs/
    brave-mango/       # actual git worktree
    add-caching/    # actual git worktree
```

Git is the source of truth. codemob reconciles its metadata against actual worktrees on every operation.

## Development

```bash
make build          # build the binary
make install        # dev install (emulates Homebrew layout at /opt/homebrew)
make test           # run integration tests
make clean          # remove build artifacts
```

## License

GPL-3.0
