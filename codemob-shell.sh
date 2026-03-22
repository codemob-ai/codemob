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
    --new-mob|--new-codemob)       shift; codemob --new "$@" ;;
    --resume-mob|--resume-codemob) shift; codemob --resume "$@" ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob --list "$@" ;;
    *)
      command claude "$@"
      codemob --check-queue
      ;;
  esac
}

codex() {
  case "${1:-}" in
    --new-mob|--new-codemob)       shift; codemob --new --agent codex "$@" ;;
    --resume-mob|--resume-codemob) shift; codemob --resume "$@" ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob --list "$@" ;;
    *)
      command codex "$@"
      codemob --check-queue
      ;;
  esac
}
