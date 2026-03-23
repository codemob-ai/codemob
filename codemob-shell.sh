#!/usr/bin/env bash
# codemob-shell.sh — optional terminal enhancement for codemob
# Source this in .zshrc/.bashrc to get:
#   - mob() alias for codemob
#   - claude/codex wrappers with --new-mob / --resume-mob flags

# Unique per-terminal session ID — used by codemob to track the last active mob
# per terminal so `--resume` can default to it without collisions.
[ -z "$CODEMOB_SESSION" ] && export CODEMOB_SESSION=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo $$)

codemob() {
  case "${1:-}" in
    cd) shift
      local dir
      dir="$(command codemob path "$@")" || return $?
      if [ "$dir" = "$PWD" ]; then
        echo "Already here."
        return 0
      fi
      cd "$dir"
      ;;
    *)  command codemob "$@" ;;
  esac
}

mob() {
  codemob "$@"
}

claude() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob new --agent claude "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob resume "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob list "$@"; return $? ;;
    *)
      command claude "$@"
      local ec=$?
      codemob check-queue 2>/dev/null
      return $ec
      ;;
  esac
}

codex() {
  case "${1:-}" in
    --mob|--codemob|--new-mob|--new-codemob) shift; codemob new --agent codex "$@"; return $? ;;
    --resume-mob|--resume-codemob)       shift; codemob resume "$@"; return $? ;;
    --switch-mob|--switch-codemob)       shift; codemob switch "$@"; return $? ;;
    --list-mob|--list-mobs|--list-codemob|--list-codemobs) shift; codemob list "$@"; return $? ;;
    *)
      command codex "$@"
      local ec=$?
      codemob check-queue 2>/dev/null
      return $ec
      ;;
  esac
}
