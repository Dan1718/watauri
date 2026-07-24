#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
TARGET_DIR="${1:-$REPO_ROOT/whatsapp-tauri}"

if [[ ! -d "$TARGET_DIR" ]]; then
  echo "Target directory does not exist: $TARGET_DIR" >&2
  exit 1
fi

echo "Local DB reset"
echo "Target: $TARGET_DIR"
echo

mapfile -d '' DB_FILES < <(
  find "$TARGET_DIR" -type f \( \
    -name '*.db' -o \
    -name '*.db-wal' -o \
    -name '*.db-shm' \
  \) -print0 | sort -z
)

if (( ${#DB_FILES[@]} == 0 )); then
  echo "No database files found."
  exit 0
fi

echo "Found ${#DB_FILES[@]} database file(s):"
for file in "${DB_FILES[@]}"; do
  printf '  - %s\n' "${file#$REPO_ROOT/}"
done
echo

removed=0
failed=0

for file in "${DB_FILES[@]}"; do
  if rm -f -- "$file"; then
    ((removed += 1))
  else
    ((failed += 1))
    printf 'Failed to remove: %s\n' "${file#$REPO_ROOT/}" >&2
  fi
done

echo "Removed: $removed"
echo "Failed:  $failed"
echo

mapfile -d '' REMAINING < <(
  find "$TARGET_DIR" -type f \( \
    -name '*.db' -o \
    -name '*.db-wal' -o \
    -name '*.db-shm' \
  \) -print0 | sort -z
)

if (( ${#REMAINING[@]} == 0 )); then
  echo "Status: clean, no database files remain under target."
else
  echo "Status: ${#REMAINING[@]} database file(s) still remain:"
  for file in "${REMAINING[@]}"; do
    printf '  - %s\n' "${file#$REPO_ROOT/}"
  done
fi
