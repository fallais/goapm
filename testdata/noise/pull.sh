#!/usr/bin/env bash
# Fetch the full noise corpus into ./full/.
#
# Source: DEMAND CC-BY-SA 3.0. https://zenodo.org/record/1227121
# Run from testdata/noise/.

set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
cd "$here"

mkdir -p full
echo "TODO: implement download of DEMAND subset (PCAFETER, OOFFICE, NPARK, etc.)"
echo "Expected: mono 16 kHz WAVs, ~30 s each, in ./full/."
