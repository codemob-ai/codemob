![codemob](img/banner.png)

Run parallel AI coding sessions in isolated git worktrees. Works transparently with Claude Code and Codex — just use them like you normally would.

## The problem

You're working with Claude Code on a feature. You need to start a second task in the same repo without losing context. You could manually create a git worktree, cd into it, launch another claude session... or:

```bash
claude --new-mob
```

That's it. codemob creates an isolated worktree, launches Claude Code inside it, and manages everything. Same for Codex:

```bash
codex --new-mob
```

## Switch between sessions — even from inside Claude

You're deep in a Claude session and want to jump to another mob. Just run `/mob-switch`, pick one, exit Claude. codemob automatically launches the new session:

```
❯ /mob-switch

⏺ Here are your other mobs:

  #  NAME             AGENT   CREATED
  1  wild-kumquat     claude  2h ago
  2  epic-apricot     codex   30m ago

  Which mob would you like to switch to?

❯ 1

⏺ Switch queued. Exit this session (Ctrl+C) and codemob will
  automatically launch the new mob.
```

Switch agents mid-session with `/mob-switch-agent` — go from Claude to Codex (or back) on the same worktree.

## Install

```bash
git clone https://github.com/codemob-ai/codemob.git
cd codemob
go build -o codemob .
# put the binary on your PATH, then:
codemob init
```

`codemob init` handles everything: shell integration, gitignore, Claude Code slash commands, permissions.

## Usage

### Start sessions

```bash
claude --new-mob                 # new mob + claude
codex --new-mob                  # new mob + codex
claude --new-mob brave-mango     # with a specific name
codemob --new                    # auto-generated name
codemob --new --agent codex      # pick agent explicitly
```

### Manage mobs

```bash
codemob --list                   # list all mobs
codemob --resume brave-mango     # resume by name
codemob --resume 2               # resume by index
codemob remove brave-mango       # remove a mob
codemob clear                    # remove all mobs
```

### From inside Claude Code / Codex

Slash commands (installed automatically by `codemob init`):

| Command | What it does |
|---|---|
| `/mob-list` | List all mobs |
| `/mob-new` | Create a new mob (launches after exit) |
| `/mob-switch` | Switch to another mob (launches after exit) |
| `/mob-switch-agent` | Swap agent mid-session (e.g., claude to codex) |
| `/mob-remove` | Remove a mob |

All commands also work with `/codemob-` prefix.

## How it works

Each mob is a git worktree under `.codemob/mobs/`. codemob manages the lifecycle and launches agents as child processes using a trampoline pattern — when you queue a switch from inside an agent, codemob picks it up after the agent exits and launches the next session automatically.

Git is the source of truth. If you manually delete a worktree, codemob cleans up its metadata on the next operation.

## Development

```bash
make build          # build the binary
make install        # dev install (emulates Homebrew layout)
make test           # run integration tests
```

## License

GPL-3.0
