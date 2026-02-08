#!/usr/bin/env bash
# ralph-stream.sh — Agentic development loop using Claude Code
# Usage: ./plans/ralph-stream.sh <implementation-file> <prd-file> <progress-file> <iterations>

set -eo pipefail

if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <implementation-file> <prd-file> <progress-file> <iterations>"
  echo ""
  echo "Arguments:"
  echo "  implementation-file  Path to the implementation/upgrade plan (e.g., plans/s-tier-upgrade.md)"
  echo "  prd-file             Path to the PRD file (e.g., plans/prd.json)"
  echo "  progress-file        Path to the progress tracking file (e.g., plans/progress.txt)"
  echo "  iterations           Number of iterations to run"
  exit 1
fi

IMPL_FILE="$1"
PRD_FILE="$2"
PROGRESS_FILE="$3"
ITERATIONS="$4"

# Validate input files exist
if [ ! -f "$IMPL_FILE" ]; then
  echo "Error: Implementation file not found: $IMPL_FILE"
  exit 1
fi

if [ ! -f "$PRD_FILE" ]; then
  echo "Error: PRD file not found: $PRD_FILE"
  exit 1
fi

# Create progress file if it doesn't exist
if [ ! -f "$PROGRESS_FILE" ]; then
  touch "$PROGRESS_FILE"
  echo "Created progress file: $PROGRESS_FILE"
fi

# Resolve to absolute paths for the @ file references
IMPL_FILE="$(cd "$(dirname "$IMPL_FILE")" && pwd)/$(basename "$IMPL_FILE")"
PRD_FILE="$(cd "$(dirname "$PRD_FILE")" && pwd)/$(basename "$PRD_FILE")"
PROGRESS_FILE="$(cd "$(dirname "$PROGRESS_FILE")" && pwd)/$(basename "$PROGRESS_FILE")"

# jq filter to extract streaming text from assistant messages
stream_text='select(.type == "assistant").message.content[]? | if .type == "text" then (.text // empty | gsub("\n"; "\r\n") | . + "\r\n\n") elif .type == "tool_use" then "\r\n⚡ [" + .name + "] " + (.input | if .command then .command elif .filePath then .filePath elif .pattern then .pattern elif .description then .description else (tostring | .[0:80]) end) + "\r\n" else empty end'

# jq filter to extract final result
final_result='select(.type == "result").result // empty'

# --- Cleanup & signal handling ---
# We run claude in a background subshell so we can kill it explicitly on
# Ctrl+C.  Bash traps on INT don't interrupt a foreground pipeline, and
# claude itself may catch/ignore SIGINT.  By backgrounding the pipeline
# and using `wait`, the trap fires immediately when a signal arrives.

CLAUDE_PID=""
TMPFILES=()
INTERRUPTED=false

cleanup() {
  if [ -n "$CLAUDE_PID" ]; then
    kill -TERM "$CLAUDE_PID" 2>/dev/null
    kill -TERM -- "-$CLAUDE_PID" 2>/dev/null || true
    wait "$CLAUDE_PID" 2>/dev/null || true
    CLAUDE_PID=""
  fi
  for f in "${TMPFILES[@]}"; do
    rm -f "$f"
  done
}

abort() {
  INTERRUPTED=true
  echo ""
  echo "Interrupted."
  cleanup
  trap - INT TERM EXIT
  kill -INT $$
}

trap cleanup EXIT
trap abort INT TERM

for ((i = 1; i <= ITERATIONS; i++)); do
  echo "=== Iteration $i of $ITERATIONS ==="

  tmpfile=$(mktemp)
  TMPFILES+=("$tmpfile")

  # Run claude in background, writing to tmpfile.
  # The subshell + pipeline runs in background; we wait on it.
  (
    claude \
      --verbose \
      --print \
      --dangerously-skip-permissions \
      --output-format stream-json \
      "@${IMPL_FILE} \
@${PRD_FILE} \
@${PROGRESS_FILE} \
1. Decide which task to work on next. \
This should be the one YOU decide has the highest priority, \
- not necessarily the first in the list. \
You can use the implementation md file for more context, but baseline tasks are in the prd. \
2. Keep changes small and focused: \
- One logical change per commit \
- If a task feels too large, break it into subtasks \
- Prefer multiple small commits over one large commit \
- Run feedback loops after each change, not at the end \
Quality over speed. Small steps compound into big progress. \
3. Check any feedback loops. Run apps/api/scripts/check.sh for full verification. \
If check.sh does not exist yet, run the checks manually in order: \
go build ./..., go vet ./..., golangci-lint run ./... (if configured), go test ./... \
Fix errors in order: compilation first, then vet, then lint, then tests. \
4. Append your progress to the progress file. \
5. Make a git commit of that feature. \
You should commit with a conventional commit message, such as feat: message, fix: message, refactor: message, etc. \
ONLY WORK ON A SINGLE FEATURE. \
Before committing, run ALL feedback loops. \
Do NOT commit if any feedback loop fails. Fix issues first. \
DO NOT commit anything in the /plans folder, nor .gitignore. \
If, while implementing the feature, you notice that all work \
is complete, output <promise>COMPLETE</promise>. \
6. After completing each task, append to the progress file: \
- Task completed and PRD item reference \
- Key decisions made and reasoning \
- Files changed \
- Any blockers or notes for next iteration \
Keep entries concise. Sacrifice grammar for the sake of concision. This file helps future iterations skip exploration. \
7. Once done, update the prd on passes.
" |
      grep --line-buffered '^{' |
      tee "$tmpfile" |
      jq --unbuffered -rj "$stream_text"
  ) &
  CLAUDE_PID=$!

  # `wait` is interruptible by trapped signals — unlike a foreground
  # pipeline.  When Ctrl+C arrives, the abort() handler fires, kills
  # the subprocess, sets INTERRUPTED=true, and re-raises SIGINT.
  # The `|| true` prevents set -e from exiting on non-zero claude exit.
  wait "$CLAUDE_PID" || true
  CLAUDE_PID=""

  # If the trap fired, abort() already called exit; but in case
  # execution continues (e.g. race), bail out explicitly.
  if $INTERRUPTED; then
    exit 130
  fi

  result=$(jq -r "$final_result" "$tmpfile" 2>/dev/null || echo "")

  if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
    echo "PRD complete, exiting."
    exit 0
  fi
done
