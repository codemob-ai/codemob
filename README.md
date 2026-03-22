![codemob](img/banner.png)

Manage isolated AI agent workspaces using git worktrees. No tmux, no terminal multiplexer, no special setup. Works in any terminal.

```bash
claude --new-mob                 # creates a worktree, launches claude in it
codex --new-mob                  # same, but with codex
codemob --list                   # see what's running
```

Switch between sessions from inside Claude using `/mob-switch`. Switch agents mid-session with `/mob-switch-agent`.

## Install

```bash
git clone https://github.com/codemob-ai/codemob.git
cd codemob
go build -o codemob .
# put the binary on your PATH, then:
codemob init
```

## Usage

```bash
# start
claude --new-mob                 # new mob + claude
codex --new-mob                  # new mob + codex
codemob --new brave-mango        # named mob, default agent
codemob --new --agent codex      # pick agent

# manage
codemob --list                   # list mobs (with indices)
codemob --resume brave-mango     # resume by name
codemob --resume 2               # resume by index
codemob remove brave-mango       # remove one
codemob clear                    # remove all
```

### Inside Claude Code / Codex

| Command | |
|---|---|
| `/mob-list` | List mobs |
| `/mob-new` | Create mob (launches after exit) |
| `/mob-switch` | Switch mob (launches after exit) |
| `/mob-switch-agent` | Swap agent (claude <-> codex) |
| `/mob-remove` | Remove mob |

Also available as `/codemob-*`.

## How it works

Each mob is a git worktree under `.codemob/mobs/`. Agents are launched as child processes. When you queue a switch from inside an agent (via slash command), codemob picks it up after exit and launches the next session.

Git is the source of truth. Stale metadata gets cleaned up automatically.

## Development

```bash
make build
make install        # dev install to /opt/homebrew
make test
```

## License

GPL-3.0
