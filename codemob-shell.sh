#!/usr/bin/env bash
# codemob-shell.sh — optional terminal enhancement for codemob
# Source this in .zshrc/.bashrc to get:
#   - mob() alias for codemob
#   - claude/codex wrappers with --new-mob / --resume-mob flags

mob() {
  codemob "$@"
}

claude() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob --new --agent claude "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob --resume "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob --switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob --list "$@"; return $? ;;
    *)
      command claude "$@"
      local ec=$?
      codemob --check-queue 2>/dev/null
      return $ec
      ;;
  esac
}

codex() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob --new --agent codex "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob --resume "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob --switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob --list "$@"; return $? ;;
    *)
      command codex "$@"
      local ec=$?
      codemob --check-queue 2>/dev/null
      return $ec
      ;;
  esac
}
