#!/bin/sh

ROOT=$(git rev-parse --show-toplevel)
SCRIPTS="$ROOT/scripts"

if [ -f "$ROOT/.env" ]; then
	set -o allexport
	source "$ROOT/dev.env"
	set +o allexport
fi

if [ -z "$OPENAI_API_KEY" ]; then
	echo "Missing OPENAI_API_KEY environment variable"
	exit 1
fi

# check if jotbot is an installed binary
if ! command -v jotbot &> /dev/null; then
	echo "'jotbot' executable not found in PATH"
	echo "Please install jotbot with"
	echo "  go install github.com/modernice/jotbot/cmd/jotbot@main"
	echo
	echo "Then run this script again."
	exit 1
fi

echo "Generating missing documentation ..."
echo

jotbot "$ROOT"
