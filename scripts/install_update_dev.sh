#!/usr/bin/env bash
# Install the development auto-update helper and systemd user timer.

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${HERO_UPDATE_INSTALL_DIR:-/home/ricardo/installable/hero}"
USER_UNIT_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/systemd/user"
UNIT_SERVICE="${USER_UNIT_DIR}/hero-update.service"
UNIT_TIMER="${USER_UNIT_DIR}/hero-update.timer"

mkdir -p "${INSTALL_DIR}" "${USER_UNIT_DIR}"
install -m 0755 "${ROOT_DIR}/scripts/hero-update.sh" "${INSTALL_DIR}/hero-update.sh"

if [[ ! -e "${INSTALL_DIR}/needs-update.txt" ]]; then
  printf 'false\n' >"${INSTALL_DIR}/needs-update.txt"
fi
chmod 0600 "${INSTALL_DIR}/needs-update.txt"

sed \
  -e "s|@HERO_UPDATE_SOURCE@|${ROOT_DIR}|g" \
  -e "s|@HERO_UPDATE_INSTALL_DIR@|${INSTALL_DIR}|g" \
  "${ROOT_DIR}/scripts/systemd/hero-update.service.in" >"${UNIT_SERVICE}"
install -m 0644 "${ROOT_DIR}/scripts/systemd/hero-update.timer" "${UNIT_TIMER}"

systemctl --user daemon-reload
systemctl --user enable --now hero-update.timer

printf 'Installed Hero auto-update helper in %s\n' "${INSTALL_DIR}"
printf 'Timer: systemctl --user status hero-update.timer\n'
printf 'Logs:  journalctl --user -u hero-update.service -f\n'
