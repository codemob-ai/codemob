![codemob](img/banner.png)

【🌕】**Terminal-agnostic AI agent workflow manager with parallel isolated sessions.**

_Powered by git worktrees under the hood, but you don't need to know that._

---

## Why

| :exclamation: Working on multiple things at once with AI agents in a single repository is a non-trivial problem. |
|---|

【🌗】`claude --worktree` solves it — creates an isolated worktree, launches a session, offers to clean up when you're done. Until you decide not to clean up, because you want to come back to it later. Then it's just a directory somewhere that you need to track down, `cd` into, and relaunch the agent in manually.

【🌕】`codemob` manages the full lifecycle — _create_, _resume_, _list_, _switch_, _clean up_.

---

> **Claude** (primary focus) and **Codex** supported out of the box.
> Other terminal-based agents work too — `codemob cd` drops you into the workspace.

## 【🌕】How

Start a new session — codemob creates an isolated workspace and drops you into your agent:

```bash
❯ claude --new-mob

  ● codemob  Created mob 'wild-kumquat' on branch mob/wild-kumquat

 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat

❯ help me refactor the auth module
```

Need another session? Create one without leaving Claude:

```bash
 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat

❯ /codemob-new

⏺ Name or auto-generate?

❯ auto

⏺ New mob queued. Exit (Ctrl+C) and codemob will create and launch it.

^C

  ● codemob  Created mob 'epic-apricot' on branch mob/epic-apricot

 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/epic-apricot
```

Switch between sessions — `/mob-switch`, pick one, exit, done:

```bash
 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/epic-apricot

❯ /mob-switch

⏺ #  NAME             LAST AGENT  CREATED
  1  wild-kumquat     claude      2h ago
  2  angry-eggplant   claude      12h ago

  Which mob?

❯ 1

⏺ Switch queued. Exit (Ctrl+C) and codemob launches the next session.

^C

  ● codemob  Switching to mob 'wild-kumquat'

 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat
```

Swap the agent on the fly — go from Claude to Codex (or back) on the same workspace:

```bash
 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat

❯ /mob-change-agent

⏺ codemob supports claude and codex. You're currently on claude.
  Switch to codex?

❯ yes

⏺ Agent switch queued. Exit (Ctrl+C).

^C

  ● codemob  Switching mob 'wild-kumquat' to agent 'codex'

╭──────────────────────────────────────────────────────╮
│ >_ OpenAI Codex                                      │
│                                                      │
│ directory: ~/my-project/.codemob/mobs/wild-kumquat   │
╰──────────────────────────────────────────────────────╯
```

## Install

Homebrew tap is WIP. For now, build from source:

```bash
git clone https://github.com/codemob-ai/codemob.git
cd codemob
make install    # builds and copies to /opt/homebrew
codemob init
```

## Usage

`codemob` and `mob` are interchangeable — use whichever you prefer.

```bash
# start
codemob new                      # auto-generated name, default agent
codemob new brave-mango          # named mob
codemob new --agent codex        # pick agent
claude --new-mob                 # shorthand, launches claude
claude --new-mob brave-mango     # shorthand with name
claude --mob                     # even shorter
codex --new-mob                  # shorthand, launches codex

# manage
codemob list                     # list mobs (with indices)
codemob resume brave-mango       # resume by name
codemob resume 2                 # resume by index
codemob remove brave-mango       # remove one
codemob purge                    # remove all
```

Shell aliases (`claude --new-mob`, `claude --mob`, `codex --new-mob`, `mob new`) also work after `codemob init`.

### Inside Claude Code / Codex

| Command | |
|---|---|
| `/codemob-list` | List mobs |
| `/codemob-new` | Create mob (launches after exit) |
| `/codemob-switch` | Switch mob (launches after exit) |
| `/codemob-change-agent` | Swap agent (claude <-> codex) |
| `/codemob-remove` | Remove mob |

Also available as `/mob-*`.

## How the agent flags work (they don't)

`--new-mob`, `--resume-mob`, and friends aren't real Claude or Codex flags. They never reach the agent.

【🌕】`codemob init` sources a small shell script into your `.zshrc` that wraps the `claude` and `codex` commands. When you type `claude --new-mob`, the wrapper intercepts the flag before Claude ever sees it and routes it to `codemob new --agent claude` instead. Any flag it doesn't recognize? Passed straight through to the real `claude` binary, untouched.

No patches, no plugins, no monkey-patching. Just a shell function pretending to be `claude` and skimming a few arguments off the top.

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
