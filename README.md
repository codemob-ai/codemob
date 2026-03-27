![codemob](img/banner.png)

<p align="center">【🌕】<b>Terminal-agnostic AI agent workflow manager with parallel isolated sessions.</b></p>

<p align="center"><i>Powered by git worktrees under the hood, but you don't need to know that.</i></p>

---

## Why

> [!WARNING]
> Working on **multiple things at once** with AI agents in a **single repository** is a non-trivial problem.

【🌗】`claude --worktree` solves it. Kind of... Creates an isolated worktree, launches a session, offers to clean up when you're done. Until you decide not to clean up, because you want to come back to it later. Then it's just a directory somewhere that you need to track down, `cd` into, and relaunch the agent manually.

【🌕】`codemob` manages the full lifecycle - _create_, _resume_, _list_, _switch_, _clean up_.

**So... if you hate `git worktree` commands and have no interest in switching to a specialized AI IDE - this is probably for you.**

<details>
<summary>✨ Supported agents</summary>
<br>

**Claude** (primary focus) and **Codex** supported out of the box.
Other terminal-based agents work too - `codemob cd` drops you into the workspace.

> Codex integration works but has rough edges around in-session slash commands (`/mob-new`, `/mob-switch`, etc.). Workspace creation, switching, and lifecycle management from the terminal all work fine. Improving this is a high priority.

</details>

## How

TBD

## Examples

Start a new session - codemob creates an isolated workspace and drops you into your agent:

```bash
❯ claude --new-codemob

  【●】codemob  Created mob 'wild-kumquat' on branch mob/wild-kumquat

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

⏺ This will end our current conversation. codemob will automatically
  close this session and launch the new one. Are you sure?

❯ yes

  【●】codemob  Session ended - mob 'wild-kumquat'

  【●】codemob  Created mob 'epic-apricot' on branch mob/epic-apricot

 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/epic-apricot
```

Switch between sessions - `/mob-switch`, pick one, done:

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

⏺ This will end our current conversation. codemob will automatically
  close this session and launch the new one. Are you sure?

❯ yes

  【●】codemob  Session ended - mob 'epic-apricot'

  【●】codemob  Switching to mob 'wild-kumquat'

 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat
```

Swap the agent on the fly - go from Claude to Codex (or back) on the same workspace:

```bash
 ▐▛███▜▌   Claude Code
▝▜█████▛▘
  ▘▘ ▝▝    ~/my-project/.codemob/mobs/wild-kumquat

❯ /mob-change-agent

⏺ codemob supports claude and codex. You're currently on claude.
  Switch to codex?

❯ yes

⏺ This will end our current conversation. codemob will automatically
  close this session and launch the new one. Are you sure?

❯ yes

  【●】codemob  Session ended - mob 'wild-kumquat'

  【●】codemob  Switching mob 'wild-kumquat' to agent 'codex'

╭──────────────────────────────────────────────────────╮
│ >_ OpenAI Codex                                      │
│                                                      │
│ directory: ~/my-project/.codemob/mobs/wild-kumquat   │
╰──────────────────────────────────────────────────────╯
```

## Install

### Homebrew (recommended)

```bash
brew tap codemob-ai/codemob
brew install codemob
codemob init
```

### From source

```bash
git clone https://github.com/codemob-ai/codemob.git
cd codemob
make install    # builds and copies to /opt/homebrew
codemob init
```

## Usage

> [!IMPORTANT]
> **`codemob` and `mob` are interchangeable everywhere** - commands, flags, slash commands.
>
> `codemob new` = `mob new`, `claude --new-codemob` = `claude --new-mob`, `/codemob-new` = `/mob-new`

```bash
# create
codemob new                      # auto-generated name, default agent
codemob new brave-mango          # named mob
codemob new --agent codex        # pick agent

# resume / open
codemob resume brave-mango       # continue previous session
codemob open brave-mango         # fresh agent session

# navigate
codemob cd brave-mango           # cd into a mob's worktree
codemob cd root                  # cd back to the main repo

# manage
codemob list                     # list mobs (with indices)
codemob resume 2                 # resume by index
codemob remove brave-mango       # remove one
codemob purge                    # remove all
```

**Claude** and **Codex** shorthands:

| | **Claude** | **Codex** |
|---|---|---|
| Create | `claude --new-codemob [name]` | `codex --new-codemob [name]` |
| Resume | `claude --resume-codemob [name]` | `codex --resume-codemob [name]` |
| Open | `claude --open-codemob [name]` | `codex --open-codemob [name]` |

`[name]` *is optional - omit it and codemob will show an interactive picker.*

### Inside Claude Code / Codex

| Command | |
|---|---|
| `/codemob-list` | List mobs |
| `/codemob-new` | Create mob (launches after exit) |
| `/codemob-switch` | Switch mob (launches after exit) |
| `/codemob-change-agent` | Swap agent (claude <-> codex) |
| `/codemob-remove` | Remove mob |

## How the agent flags work (they don't)

```bash
❯ claude --new-codemob
```

`--new-codemob`, `--resume-codemob`, and friends aren't real Claude or Codex flags. They never reach the agent.

【🌕】`codemob init` sources a small shell script into your shell RC file (`.zshrc`, `.bashrc`, or `.bash_profile`) that wraps the `claude` and `codex` commands. When you type `claude --new-codemob`, the wrapper intercepts the flag before Claude ever sees it and routes it to `codemob new --agent claude` instead. Any flag it doesn't recognize? Passed straight through to the real `claude` binary, untouched.

No patches, no plugins, no monkey-patching. Just a shell function pretending to be `claude` and skimming a few arguments off the top.

## Hooks

### `post_create_script`

Run a shell script automatically after every `codemob new`. The script runs with `cwd` set to the new worktree - before the agent launches.

Set it in `.codemob/config.json` (absolute or relative to repo root):

```json
{
  "post_create_script": "./scripts/mob-setup.sh"
}
```

Example - install dependencies so the agent starts in a working environment:

```sh
#!/bin/sh
npm install --silent
bundle install --quiet
```

If the script exits non-zero, the mob is cleaned up and the agent is not launched.

## Under the hood

【🌕】Each mob is a git worktree under `.codemob/mobs/`. Agents are launched as child processes. When you queue an action from inside an agent (via slash command), codemob detects it, terminates the agent, and launches the next session automatically.

Git is the source of truth. Stale metadata gets cleaned up automatically.

## Development

```bash
make build
make install        # dev install to /opt/homebrew
make test
```

## License

GPL-3.0
