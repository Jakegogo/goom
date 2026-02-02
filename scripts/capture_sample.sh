#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   scripts/capture_sample.sh
#   WAIT_SECS=5 SAMPLE_SECS=15 OUT=/tmp/goom-argdump-sample.txt scripts/capture_sample.sh
#
# Optional env vars:
#   GO_BIN        - Go binary to use (default: /Users/jake/sdk/go1.18.10/bin/go)
#   PKG           - Package to test (default: ./internal/argdump)
#   TEST_REGEX    - go test -run regex (default: ^Test_Patched_PreGo124_MultiReturn_NoDump$)
#   GO_TEST_TIMEOUT - go test -timeout value (default: 10m)
#   WAIT_SECS       - seconds to wait before sampling (default: 6)
#   SAMPLE_SECS     - seconds to sample (default: 15)
#   OUT             - output file for sample (default: /tmp/goom-argdump-sample.txt)
#   LOOP_MAX      - max attempts to reproduce a hang (default: 20)
#   GORO_OUT      - goroutine dump output file (default: /tmp/goom-argdump-goroutines.txt)
#   KILL_AFTER    - kill test process after sampling (default: 1)
#
# Example (force local NoDump repro):
#   ARGDUMP_REPRO_LOCAL_NODUMP=1 scripts/capture_sample.sh

GO_BIN="${GO_BIN:-/Users/jake/sdk/go1.18.10/bin/go}"
PKG="${PKG:-./internal/argdump}"
TEST_REGEX="${TEST_REGEX:-^Test_Patched_PreGo124_MultiReturn_NoDump$}"
GO_TEST_TIMEOUT="${GO_TEST_TIMEOUT:-10m}"
PKG_BASENAME="$(basename "$PKG")"
WAIT_SECS="${WAIT_SECS:-6}"
SAMPLE_SECS="${SAMPLE_SECS:-15}"
OUT="${OUT:-/tmp/goom-argdump-sample.txt}"
LOOP_MAX="${LOOP_MAX:-20}"
KILL_AFTER="${KILL_AFTER:-1}"
GORO_OUT="${GORO_OUT:-/tmp/goom-argdump-goroutines.txt}"

echo "[capture] go: $GO_BIN"
echo "[capture] pkg: $PKG"
echo "[capture] test: $TEST_REGEX"
echo "[capture] timeout: $GO_TEST_TIMEOUT"
echo "[capture] wait: ${WAIT_SECS}s sample: ${SAMPLE_SECS}s out: $OUT"
echo "[capture] loop max: $LOOP_MAX"
echo "[capture] goroutine dump: $GORO_OUT"

mkdir -p "$(dirname "$OUT")"
cat >"$OUT" <<EOF
goom argdump sample
time: $(date)
cmd: GOTRACEBACK=all $GO_BIN test $PKG -gcflags=all=-l -run $TEST_REGEX -v -timeout $GO_TEST_TIMEOUT -count=1
EOF

attempt=1
while [[ "$attempt" -le "$LOOP_MAX" ]]; do
  echo "[capture] attempt $attempt/$LOOP_MAX"
  if [[ "$attempt" -eq 1 ]]; then
    : >"$GORO_OUT"
  else
    printf "\n---- attempt %d ----\n" "$attempt" >>"$GORO_OUT"
  fi
  set +e
  GOTRACEBACK=all "$GO_BIN" test "$PKG" -gcflags=all=-l -run "$TEST_REGEX" -v -timeout "$GO_TEST_TIMEOUT" -count=1 \
    >"$GORO_OUT" 2>&1 &
  pid=$!
  set -e
  echo "[capture] pid=$pid"

  sleep "$WAIT_SECS"

  if kill -0 "$pid" 2>/dev/null; then
    echo "[capture] process still running after ${WAIT_SECS}s; sampling..."
    {
      echo "[capture] ps snapshot:"
      ps -ax -o pid,ppid,pgid,comm | grep -E 'go$|argdump\.test' || true
    } >>"$GORO_OUT"
    target_pid=""
    if command -v pgrep >/dev/null 2>&1; then
      for cand in $(pgrep -f "${PKG_BASENAME}\\.test" || true); do
        p="$cand"
        while [[ -n "$p" && "$p" -gt 1 ]]; do
          if [[ "$p" == "$pid" ]]; then
            target_pid="$cand"
            break
          fi
          p="$(ps -o ppid= -p "$p" | tr -d ' ')"
        done
        [[ -n "$target_pid" ]] && break
      done
    fi
    if [[ -z "$target_pid" ]]; then
      child="$(ps -o pid= -ppid "$pid" | head -n 1 | tr -d ' ')"
      while [[ -n "$child" ]]; do
        target_pid="$child"
        child="$(ps -o pid= -ppid "$child" | head -n 1 | tr -d ' ')"
      done
    fi
    if [[ -z "$target_pid" ]]; then
      target_pid="$pid"
      echo "[capture] no child pid detected; using parent pid=$pid"
    else
      echo "[capture] detected test pid=$target_pid"
    fi

    echo "[capture] sending SIGQUIT for goroutine dump to pid=$target_pid..."
    kill -QUIT "$target_pid" 2>/dev/null || true
    sleep 2
    echo "attempt $attempt: sampled running process (pid=$target_pid)" >>"$OUT"
    /usr/bin/sample "$target_pid" "$SAMPLE_SECS" -file "$OUT" || true
    echo "[capture] sample saved to $OUT"
    if [[ "$KILL_AFTER" == "1" ]]; then
      echo "[capture] killing pid=$pid"
      kill -9 "$pid" 2>/dev/null || true
    fi
    exit 0
  fi

  echo "[capture] process exited before sampling"
  echo "attempt $attempt: process exited before sampling" >>"$OUT"
  attempt=$((attempt + 1))
done

echo "[capture] no hang observed after $LOOP_MAX attempts"
