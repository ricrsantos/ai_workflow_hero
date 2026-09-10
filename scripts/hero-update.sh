#!/usr/bin/env bash
# scripts/hero-update.sh — Apply a queued local Hero development update.
#
# The script is copied next to the installed Hero binary. It is intended to be
# run by the hero-update.service systemd user unit, not by the Telegram model.

set -Eeuo pipefail

UPDATE_DIR="${HERO_UPDATE_INSTALL_DIR:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)}"
SOURCE_DIR="${HERO_UPDATE_SOURCE:-}"
BUILD_SCRIPT="${HERO_UPDATE_BUILD_SCRIPT:-${SOURCE_DIR}/scripts/build_update.sh}"
HERO_BIN="${HERO_UPDATE_BIN:-${UPDATE_DIR}/hero}"
STATE_FILE="${HERO_UPDATE_STATE:-${UPDATE_DIR}/needs-update.txt}"
LOCK_FILE="${HERO_UPDATE_LOCK:-${UPDATE_DIR}/hero-update.lock}"
BACKUP_FILE="${HERO_UPDATE_BACKUP:-${UPDATE_DIR}/hero.previous}"

# The Telegram plugin is optional. Its installed daemon and manifest are
# updated only when either artifact already exists; a normal Hero update must
# never opt a user into the plugin.
TELEGRAM_PLUGIN_DIR="${HERO_UPDATE_TELEGRAM_PLUGIN_DIR:-}"
TELEGRAM_DAEMON_BIN="${HERO_UPDATE_TELEGRAM_DAEMON:-}"
if [[ -z "${TELEGRAM_PLUGIN_DIR}" && -n "${TELEGRAM_DAEMON_BIN}" ]]; then
  TELEGRAM_PLUGIN_DIR="$(dirname -- "${TELEGRAM_DAEMON_BIN}")"
fi
if [[ -z "${TELEGRAM_PLUGIN_DIR}" && -n "${HOME:-}" ]]; then
  TELEGRAM_PLUGIN_DIR="${HOME}/.workflow-hero/plugins/telegram"
fi
if [[ -z "${TELEGRAM_DAEMON_BIN}" && -n "${TELEGRAM_PLUGIN_DIR}" ]]; then
  TELEGRAM_DAEMON_BIN="${TELEGRAM_PLUGIN_DIR}/hero-telegram-daemon"
fi
TELEGRAM_MANIFEST="${HERO_UPDATE_TELEGRAM_MANIFEST:-}"
if [[ -z "${TELEGRAM_MANIFEST}" && -n "${TELEGRAM_PLUGIN_DIR}" ]]; then
  TELEGRAM_MANIFEST="${TELEGRAM_PLUGIN_DIR}/manifest.json"
fi
TELEGRAM_PID_FILE="${HERO_UPDATE_TELEGRAM_PID:-}"
if [[ -z "${TELEGRAM_PID_FILE}" && -n "${HOME:-}" ]]; then
  TELEGRAM_PID_FILE="${HOME}/.workflow-hero/run/telegram.pid"
fi

SIGNALLED_COUNT=0
MATCHING_COUNT=0
DAEMON_PIDS=()
DAEMON_PID_FALLBACK=""
PLUGIN_INSTALLED=0

NEXT_FILE=""
NEXT_DAEMON_FILE=""
DAEMON_PLUGIN_NEXT=""
MANIFEST_NEXT=""
NEW_STATE=""
HERO_BACKUP_NEXT=""
HERO_OLD=""
OLD_BACKUP=""
DAEMON_OLD=""
MANIFEST_OLD=""

HERO_OLD_PRESENT=0
BACKUP_OLD_PRESENT=0
DAEMON_OLD_PRESENT=0
MANIFEST_OLD_PRESENT=0
BACKUP_REPLACED=0
HERO_REPLACED=0
DAEMON_REPLACED=0
MANIFEST_REPLACED=0
INSTALL_COMMITTED=0

log() {
  printf '[hero-update] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

path_exists() {
  [[ -e "$1" || -L "$1" ]]
}

is_tui_process() {
  local proc="$1"
  local args=()
  local arg

  if [[ ! -r "${proc}/cmdline" ]]; then
    return 0
  fi
  mapfile -d '' args <"${proc}/cmdline" || true
  (( ${#args[@]} > 0 )) || return 0
  for arg in "${args[@]:1}"; do
    case "${arg}" in
      ""|--verbose|-v|--debug)
        ;;
      tui|chat)
        return 0
        ;;
      *)
        return 1
        ;;
    esac
  done
  return 0
}

process_executable_matches() {
  local proc="$1"
  local target="$2"
  local exe

  [[ -n "${target}" ]] || return 1
  exe="$(readlink "${proc}/exe" 2>/dev/null || true)"
  [[ "${exe}" == "${target}" || "${exe}" == "${target} (deleted)" ]]
}

collect_matching_pids() {
  local target="$1"
  local proc pid

  [[ -n "${target}" && -d /proc ]] || return 0
  for proc in /proc/[0-9]*; do
    [[ -d "${proc}" ]] || continue
    pid="${proc##*/}"
    [[ "${pid}" == "$$" ]] && continue
    if process_executable_matches "${proc}" "${target}" && kill -0 "${pid}" 2>/dev/null; then
      DAEMON_PIDS+=("${pid}")
    fi
  done
}

pid_is_listed() {
  local wanted="$1"
  local pid
  for pid in "${DAEMON_PIDS[@]}"; do
    [[ "${pid}" == "${wanted}" ]] && return 0
  done
  return 1
}

capture_daemon_pidfile() {
  local pid
  [[ -n "${TELEGRAM_PID_FILE}" && -r "${TELEGRAM_PID_FILE}" ]] || return 0
  pid="$(sed -n '1p' "${TELEGRAM_PID_FILE}" 2>/dev/null || true)"
  if [[ "${pid}" =~ ^[1-9][0-9]*$ ]] && (( pid > 1 )); then
    if ! pid_is_listed "${pid}"; then
      DAEMON_PID_FALLBACK="${pid}"
    fi
  else
    log "ignoring invalid Telegram daemon pid file"
  fi
}

count_matching_processes() {
  local target="$1"
  local label="$2"
  local proc pid

  MATCHING_COUNT=0
  [[ -n "${target}" && -d /proc ]] || return 0
  for proc in /proc/[0-9]*; do
    [[ -d "${proc}" ]] || continue
    pid="${proc##*/}"
    [[ "${pid}" == "$$" ]] && continue
    if process_executable_matches "${proc}" "${target}"; then
      if [[ "${label}" == "Hero restart" ]] && ! is_tui_process "${proc}"; then
        continue
      fi
      if kill -0 "${pid}" 2>/dev/null; then
        MATCHING_COUNT=$((MATCHING_COUNT + 1))
      fi
    fi
  done
}

signal_matching_processes() {
  local target="$1"
  local signal_name="$2"
  local label="$3"
  local proc pid

  SIGNALLED_COUNT=0
  [[ -n "${target}" && -d /proc ]] || return 0
  for proc in /proc/[0-9]*; do
    [[ -d "${proc}" ]] || continue
    pid="${proc##*/}"
    [[ "${pid}" == "$$" ]] && continue
    if process_executable_matches "${proc}" "${target}"; then
      if [[ "${label}" == "Hero restart" ]] && ! is_tui_process "${proc}"; then
        continue
      fi
      if kill -0 "${pid}" 2>/dev/null && kill "-${signal_name}" "${pid}" 2>/dev/null; then
        SIGNALLED_COUNT=$((SIGNALLED_COUNT + 1))
        log "${label} signal sent to PID ${pid}"
      fi
    fi
  done
}

signal_captured_daemons() {
  local pid
  for pid in "${DAEMON_PIDS[@]}"; do
    if [[ -d /proc ]] && ! process_executable_matches "/proc/${pid}" "${TELEGRAM_DAEMON_BIN}"; then
      continue
    fi
    if kill -0 "${pid}" 2>/dev/null && kill -TERM "${pid}" 2>/dev/null; then
      log "Telegram daemon stop signal sent to PID ${pid}"
    fi
  done
  if [[ -n "${DAEMON_PID_FALLBACK}" ]]; then
    pid="${DAEMON_PID_FALLBACK}"
    if [[ -d /proc ]] && ! process_executable_matches "/proc/${pid}" "${TELEGRAM_DAEMON_BIN}"; then
      return 0
    fi
    if kill -0 "${pid}" 2>/dev/null && kill -TERM "${pid}" 2>/dev/null; then
      log "Telegram daemon stop signal sent to PID ${pid}"
    fi
  fi
}

captured_daemon_is_alive() {
  local pid="$1"
  if [[ -d /proc ]]; then
    process_executable_matches "/proc/${pid}" "${TELEGRAM_DAEMON_BIN}" && kill -0 "${pid}" 2>/dev/null
    return
  fi
  kill -0 "${pid}" 2>/dev/null
}

wait_for_captured_daemons() {
  local deadline pid alive
  deadline=$((SECONDS + 3))
  while (( SECONDS < deadline )); do
    alive=0
    for pid in "${DAEMON_PIDS[@]}"; do
      if captured_daemon_is_alive "${pid}"; then
        alive=1
        break
      fi
    done
    if [[ -n "${DAEMON_PID_FALLBACK}" ]] && captured_daemon_is_alive "${DAEMON_PID_FALLBACK}"; then
      alive=1
    fi
    (( alive == 0 )) && return 0
    sleep 0.05
  done
  log "Telegram daemon did not stop within the restart grace period"
}

notify_registered_tuis() {
  local output
  if output="$("${HERO_BIN}" internal auto-update-restart 2>&1)"; then
    log "restart request broadcast through the Telegram daemon"
    return 0
  fi
  log "Telegram daemon restart request unavailable; continuing after binary installation"
  [[ -z "${output}" ]] || log "${output}"
  return 1
}

restore_file() {
  local backup="$1"
  local destination="$2"
  local was_present="$3"
  local temporary

  if (( was_present )); then
    temporary="$(mktemp "${destination}.rollback.XXXXXX")" || {
      log "WARNING: could not create rollback file for ${destination}"
      return 1
    }
    if ! cp -p -- "${backup}" "${temporary}" || ! mv -f -- "${temporary}" "${destination}"; then
      rm -f -- "${temporary}"
      log "WARNING: could not restore ${destination}"
      return 1
    fi
  else
    rm -f -- "${destination}" || {
      log "WARNING: could not remove partially installed ${destination}"
      return 1
    }
  fi
  return 0
}

rollback() {
  set +e
  log "rolling back incomplete update"
  if (( HERO_REPLACED )); then
    restore_file "${HERO_OLD}" "${HERO_BIN}" "${HERO_OLD_PRESENT}"
  fi
  if (( BACKUP_REPLACED )); then
    restore_file "${OLD_BACKUP}" "${BACKUP_FILE}" "${BACKUP_OLD_PRESENT}"
  fi
  if (( MANIFEST_REPLACED )); then
    restore_file "${MANIFEST_OLD}" "${TELEGRAM_MANIFEST}" "${MANIFEST_OLD_PRESENT}"
  fi
  if (( DAEMON_REPLACED )); then
    restore_file "${DAEMON_OLD}" "${TELEGRAM_DAEMON_BIN}" "${DAEMON_OLD_PRESENT}"
  fi
}

cleanup() {
  local status=$?
  local temporary
  trap - EXIT
  if (( ! INSTALL_COMMITTED )); then
    rollback
  fi
  for temporary in "${NEXT_FILE}" "${NEXT_DAEMON_FILE}" "${DAEMON_PLUGIN_NEXT}" "${MANIFEST_NEXT}" "${NEW_STATE}" "${HERO_BACKUP_NEXT}" "${HERO_OLD}" "${OLD_BACKUP}" "${DAEMON_OLD}" "${MANIFEST_OLD}"; do
    [[ -n "${temporary}" ]] && rm -f -- "${temporary}"
  done
  exit "${status}"
}

[[ -d "${UPDATE_DIR}" ]] || fail "install directory does not exist: ${UPDATE_DIR}"
[[ -n "${SOURCE_DIR}" ]] || fail "HERO_UPDATE_SOURCE is not configured"
[[ -d "${SOURCE_DIR}" ]] || fail "source directory does not exist: ${SOURCE_DIR}"
[[ -x "${BUILD_SCRIPT}" ]] || fail "build_update.sh is not executable: ${BUILD_SCRIPT}"
[[ -f "${STATE_FILE}" ]] || exit 0

exec 9>"${LOCK_FILE}"
flock -n 9 || {
  log "another update is already running"
  exit 0
}

FLAG="$(sed -n '1p' "${STATE_FILE}" 2>/dev/null || true)"
[[ "${FLAG}" == "true" ]] || exit 0

[[ -x "${HERO_BIN}" ]] || fail "installed Hero binary does not exist: ${HERO_BIN}"

if [[ -n "${TELEGRAM_PLUGIN_DIR}" ]] && { path_exists "${TELEGRAM_DAEMON_BIN}" || path_exists "${TELEGRAM_MANIFEST}"; }; then
  PLUGIN_INSTALLED=1
  log "Telegram plugin detected; its daemon and manifest will be updated with Hero"
fi

# Capture the old daemon before replacing its executable. Once the rename is
# complete Linux reports its /proc executable as `(deleted)`; captured PIDs let
# us stop only the old daemon and never a freshly respawned one.
if (( PLUGIN_INSTALLED )); then
  collect_matching_pids "${TELEGRAM_DAEMON_BIN}"
  capture_daemon_pidfile
fi

trap cleanup EXIT
NEXT_FILE="$(mktemp "${UPDATE_DIR}/.hero.next.XXXXXX")"
NEXT_DAEMON_FILE="$(mktemp "${UPDATE_DIR}/.hero-telegram-daemon.next.XXXXXX")"

log "building Hero and Telegram daemon updates from ${SOURCE_DIR}"
HERO_UPDATE_OUTPUT="${NEXT_FILE}" \
HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT="${NEXT_DAEMON_FILE}" \
"${BUILD_SCRIPT}"
[[ -s "${NEXT_FILE}" && -x "${NEXT_FILE}" ]] || fail "build_update.sh did not create an executable Hero update"
[[ -s "${NEXT_DAEMON_FILE}" && -x "${NEXT_DAEMON_FILE}" ]] || fail "build_update.sh did not create an executable Telegram daemon update"

VERSION_OUTPUT="$("${NEXT_FILE}" version 2>&1)" || fail "new Hero binary failed the version check"
NEW_VERSION="$(printf '%s\n' "${VERSION_OUTPUT}" | sed -n 's/^hero version //p' | head -n1)"
[[ -n "${NEW_VERSION}" ]] || fail "new Hero binary returned an unparseable version"

HERO_OLD="$(mktemp "${UPDATE_DIR}/.hero.old.XXXXXX")"
cp -p -- "${HERO_BIN}" "${HERO_OLD}"
HERO_OLD_PRESENT=1
HERO_BACKUP_NEXT="$(mktemp "${UPDATE_DIR}/.hero.previous.next.XXXXXX")"
cp -p -- "${HERO_BIN}" "${HERO_BACKUP_NEXT}"
if path_exists "${BACKUP_FILE}"; then
  OLD_BACKUP="$(mktemp "${UPDATE_DIR}/.hero.previous.old.XXXXXX")"
  cp -p -- "${BACKUP_FILE}" "${OLD_BACKUP}"
  BACKUP_OLD_PRESENT=1
fi

if (( PLUGIN_INSTALLED )); then
  [[ -d "${TELEGRAM_PLUGIN_DIR}" ]] || fail "Telegram plugin directory does not exist: ${TELEGRAM_PLUGIN_DIR}"
  command -v python3 >/dev/null 2>&1 || fail "python3 is required to update the Telegram plugin manifest"

  if path_exists "${TELEGRAM_DAEMON_BIN}"; then
    DAEMON_OLD="$(mktemp "${UPDATE_DIR}/.telegram-daemon.old.XXXXXX")"
    cp -p -- "${TELEGRAM_DAEMON_BIN}" "${DAEMON_OLD}"
    DAEMON_OLD_PRESENT=1
  fi
  if path_exists "${TELEGRAM_MANIFEST}"; then
    MANIFEST_OLD="$(mktemp "${UPDATE_DIR}/.telegram-manifest.old.XXXXXX")"
    cp -p -- "${TELEGRAM_MANIFEST}" "${MANIFEST_OLD}"
    MANIFEST_OLD_PRESENT=1
  fi

  DAEMON_PLUGIN_NEXT="$(mktemp "${TELEGRAM_PLUGIN_DIR}/.hero-telegram-daemon.next.XXXXXX")"
  install -m 0755 "${NEXT_DAEMON_FILE}" "${DAEMON_PLUGIN_NEXT}"
  MANIFEST_NEXT="$(mktemp "${TELEGRAM_PLUGIN_DIR}/.manifest.next.XXXXXX")"
  python3 - "${TELEGRAM_MANIFEST}" "${MANIFEST_NEXT}" "${NEW_VERSION}" "${TELEGRAM_DAEMON_BIN}" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

manifest_path, output_path, version, daemon_path = sys.argv[1:5]
manifest = {}
if os.path.lexists(manifest_path):
    with open(manifest_path, encoding="utf-8") as handle:
        manifest = json.load(handle)
if not isinstance(manifest, dict):
    raise ValueError("Telegram manifest must contain a JSON object")
manifest.setdefault("name", "telegram")
manifest.setdefault("protocol_version", 1)
manifest["version"] = version
manifest["daemon_path"] = daemon_path
manifest["installed_at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump(manifest, handle, indent=2)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.chmod(output_path, 0o644)
PY
fi

# Each rename is atomic within its destination directory. The staged backups
# and rollback trap make the multi-artifact update fail closed if a later
# replacement fails.
if (( PLUGIN_INSTALLED )); then
  mv -f -- "${DAEMON_PLUGIN_NEXT}" "${TELEGRAM_DAEMON_BIN}"
  DAEMON_REPLACED=1
  mv -f -- "${MANIFEST_NEXT}" "${TELEGRAM_MANIFEST}"
  MANIFEST_REPLACED=1
fi
mv -f -- "${HERO_BACKUP_NEXT}" "${BACKUP_FILE}"
BACKUP_REPLACED=1
mv -f -- "${NEXT_FILE}" "${HERO_BIN}"
HERO_REPLACED=1
chmod 0755 "${HERO_BIN}"
INSTALL_COMMITTED=1
log "installed new Hero binary; previous version saved at ${BACKUP_FILE}"
if (( PLUGIN_INSTALLED )); then
  log "installed Telegram daemon and manifest (version=${NEW_VERSION})"
fi

# SIGUSR2 is the authoritative local restart path. Stop the old Telegram
# daemon first so a restarted TUI cannot reconnect to its deleted old inode.
# The IPC fallback is attempted while that old daemon is still available when
# no matching TUI is visible through /proc.
count_matching_processes "${HERO_BIN}" "Hero restart"
hero_instances="${MATCHING_COUNT}"
restart_notified=0
if (( hero_instances == 0 )); then
  if notify_registered_tuis; then
    restart_notified=1
  fi
fi
if (( PLUGIN_INSTALLED )); then
  signal_captured_daemons
  wait_for_captured_daemons
fi
if (( hero_instances > 0 )); then
  signal_matching_processes "${HERO_BIN}" USR2 "Hero restart"
elif (( restart_notified == 0 )); then
  log "no directly discoverable Hero TUI was restarted"
fi

# Atomic write: the timer can never observe a partially written flag. If this
# final bookkeeping fails, the installed artifacts remain valid and the flag
# stays armed for a retry.
NEW_STATE="$(mktemp "${STATE_FILE}.XXXXXX")"
printf 'false\n' >"${NEW_STATE}"
chmod 0600 "${NEW_STATE}"
mv -f -- "${NEW_STATE}" "${STATE_FILE}"

log "update completed"
trap - EXIT
cleanup
