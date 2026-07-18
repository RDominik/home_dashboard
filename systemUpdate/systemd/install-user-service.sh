#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../" && pwd)"
SYSTEMUPDATE_DIR="${PROJECT_ROOT}/systemUpdate"
BIN_DIR="${HOME}/.local/bin"
BIN_PATH="${BIN_DIR}/webgui-systemupdate"
USER_UNIT_DIR="${HOME}/.config/systemd/user"
UNIT_NAME="webgui-systemupdate.service"
UNIT_TEMPLATE="${SYSTEMUPDATE_DIR}/systemd/${UNIT_NAME}"
UNIT_TARGET="${USER_UNIT_DIR}/${UNIT_NAME}"

if ! command -v go >/dev/null 2>&1; then
  echo "Go ist nicht installiert oder nicht im PATH." >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemctl ist nicht verfügbar." >&2
  exit 1
fi

mkdir -p "${BIN_DIR}" "${USER_UNIT_DIR}"

echo "[1/4] Build binary -> ${BIN_PATH}"
(
  cd "${SYSTEMUPDATE_DIR}"
  go build -o "${BIN_PATH}" .
)

echo "[2/4] Install user unit -> ${UNIT_TARGET}"
cp "${UNIT_TEMPLATE}" "${UNIT_TARGET}"

# Set the repository path in the installed unit file to this checkout location.
sed -i "s|%h/Repository/webgui|${PROJECT_ROOT}|g" "${UNIT_TARGET}"

echo "[3/4] Reload user daemon"
systemctl --user daemon-reload

echo "[4/4] Enable + start ${UNIT_NAME}"
systemctl --user enable --now "${UNIT_NAME}"

echo
echo "Fertig. Status prüfen mit:"
echo "  systemctl --user status ${UNIT_NAME}"
echo "Logs ansehen mit:"
echo "  journalctl --user -u ${UNIT_NAME} -f"