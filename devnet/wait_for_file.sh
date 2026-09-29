#!/bin/bash

set -eu

# Poll until TARGET exists. If TARGET is a directory, wait until it contains
# at least one *.json file. Used to wait for deploy artifacts before downstream
# steps so we don't have to guess timings.

TARGET="${1:?usage: wait_for_file.sh <path> [timeout_secs]}"
TIMEOUT_SECS="${2:-${TIMEOUT_SECS:-180}}"
INTERVAL_SECS="${INTERVAL_SECS:-2}"

start_ts=$(date +%s)
while :; do
  if [ -e "$TARGET" ]; then
    if [ -d "$TARGET" ]; then
      if compgen -G "$TARGET/*.json" >/dev/null 2>&1; then
        echo "wait_for_file: ready $TARGET (has *.json)"
        exit 0
      fi
    elif [ -f "$TARGET" ] && [ -s "$TARGET" ]; then
      echo "wait_for_file: ready $TARGET"
      exit 0
    fi
  fi
  now_ts=$(date +%s)
  if [ $((now_ts - start_ts)) -ge "$TIMEOUT_SECS" ]; then
    echo "wait_for_file: timeout after ${TIMEOUT_SECS}s waiting for $TARGET" >&2
    exit 1
  fi
  sleep "$INTERVAL_SECS"
done
