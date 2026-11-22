#!/usr/bin/env bash
# Rebuild the three pages and click through each in a headless browser at desktop and phone width.
set -euo pipefail
cd "$(dirname "$0")"
python3 build.py
../../scripts/browser experiments/E-002-look-and-feel/check.py
