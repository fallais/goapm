#!/usr/bin/env bash
# Fetch room impulse responses into ./full/.
#
# Run from testdata/irs/.

set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
cd "$here"

mkdir -p full
echo "TODO: download and trim a representative subset of room IRs."
echo "Target: mono 16 kHz, ~0.5-2 s, RT60 spanning 100-600 ms."
