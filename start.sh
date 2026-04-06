#!/bin/bash

# MDM Mock Service launcher
#
# Usage:
#   ./start.sh [PORT] -o [USER_COUNT]              # O365 / Graph API mock
#   ./start.sh [PORT] -j [COMPUTER_COUNT]          # Jamf Pro API mock
#   ./start.sh [PORT] -j [COMPUTER_COUNT] --dupes  # Jamf with ~10% duplicate serials
#
# Examples:
#   ./start.sh 8080 -o 1000         Graph API, 1000 users on port 8080
#   ./start.sh 8080 -j 500          Jamf API, 500 computers on port 8080
#   ./start.sh 8080 -j 10000 --dupes  Jamf API, 10k computers, ~1000 with duplicate serials
#   ./start.sh -o                   O365 mode, default port 8080, default 100 users

PORT="${1:-8080}"
shift  # consume port

MODE=""
COUNT=""
GODUPES=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      MODE="o365"
      if [[ -n "$2" && "$2" != --* && "$2" != -* ]]; then
        COUNT="$2"
        shift
      fi
      shift
      ;;
    -j)
      MODE="jamf"
      if [[ -n "$2" && "$2" != --* && "$2" != -* ]]; then
        COUNT="$2"
        shift
      fi
      shift
      ;;
    --dupes)
      GODUPES="-dupes"
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      shift
      ;;
  esac
done

if [[ -z "$MODE" ]]; then
  echo "MDM Mock Service"
  echo ""
  echo "Usage:"
  echo "  ./start.sh [PORT] -o [USER_COUNT]              O365 / Graph API mock"
  echo "  ./start.sh [PORT] -j [COMPUTER_COUNT]          Jamf Pro API mock"
  echo "  ./start.sh [PORT] -j [COMPUTER_COUNT] --dupes  Jamf with duplicate serials"
  echo ""
  echo "Examples:"
  echo "  ./start.sh 8080 -o 1000           Graph API, 1000 users"
  echo "  ./start.sh 8080 -j 500            Jamf API, 500 computers"
  echo "  ./start.sh 8080 -j 10000 --dupes  Jamf API, 10k computers, ~10% duplicate serials"
  exit 1
fi

echo "Starting MDM Mock Service..."
echo "  Port : $PORT"
echo "  Mode : $MODE"
if [[ -n "$COUNT" ]]; then
  echo "  Count: $COUNT"
fi
if [[ -n "$GODUPES" ]]; then
  echo "  Dupes: enabled (~10% of computers will share a serial number)"
fi
echo ""
echo "Press Ctrl+C to stop"
echo ""

GO_ARGS="-mode=$MODE"
[[ -n "$COUNT" ]] && GO_ARGS="$GO_ARGS -count=$COUNT"
[[ -n "$GODUPES" ]] && GO_ARGS="$GO_ARGS $GODUPES"

PORT=$PORT go run main.go $GO_ARGS

