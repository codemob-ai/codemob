![codemob](img/banner.png)

Terminal-agnostic AI agent workspace manager with parallel isolated sessions.
Powered by git worktrees under the hood, but you don't need to know that.

Start a new session — codemob creates an isolated workspace and drops you into Claude:

```
~/my-project
❯ claude --new-mob

  ● codemob  Created mob 'wild-kumquat' on branch mob/wild-kumquat

 ▐▛███▜▌   Claude Code
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat

❯ help me refactor the auth module
```

Need another session? Create one without leaving Claude:

```
❯ /codemob-new

⏺ Name or auto-generate? → auto

⏺ New mob queued. Exit (Ctrl+C) and codemob will create and launch it.

❯ Ctrl+C

  ● codemob  Created mob 'epic-apricot' on branch mob/epic-apricot

 ▐▛███▜▌   Claude Code
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/epic-apricot
```

Switch between sessions — `/mob-switch`, pick one, exit, done:

```
❯ /mob-switch

⏺ #  NAME             LAST AGENT  CREATED
  1  wild-kumquat     claude      2h ago

  Which mob? → 1

⏺ Switch queued. Exit (Ctrl+C) and codemob launches the next session.

❯ Ctrl+C

  ● codemob  Switching to mob 'wild-kumquat'

 ▐▛███▜▌   Claude Code
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat
```

Swap the agent on the fly — go from Claude to Codex (or back) on the same workspace:

```
❯ /mob-change-agent

⏺ codemob supports claude and codex. You're currently on claude.
  Switch to codex? → yes

⏺ Agent switch queued. Exit (Ctrl+C).

❯ Ctrl+C

  ● codemob  Switching mob 'wild-kumquat' to agent 'codex'

 codex                ~/my-project/.codemob/mobs/wild-kumquat
```

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
| `/mob-change-agent` | Swap agent (claude <-> codex) |
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
