# Changelog

## 1.1.0

- Interactive `codemob --resume` — shows a picker when no mob specified, auto-selects if only one
- `codemob reinit` — re-configure base branch and default agent without losing existing mobs
- `codemob init` now prompts for default agent (claude/codex)
- Index-based mob selection — `codemob --resume 2`, `codemob remove 1`
- Current mob marked with `◀` in list output
- Worktree-aware Claude sessions — injected system prompt keeps Claude and subagents in the correct worktree
- Agents launched from worktrees get `--add-dir` access to the parent repo
- macOS Gatekeeper fix for dev installs

## 1.0.0

Initial release — core mob lifecycle, trampoline-based session switching, Claude slash commands, Codex custom prompts, shell wrappers.
