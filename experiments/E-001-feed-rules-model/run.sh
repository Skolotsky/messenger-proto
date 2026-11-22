#!/usr/bin/env bash
# Reruns E-001: the scenarios, then the simulated team. Writes results.txt and results.json next to this file.
set -uo pipefail
cd "$(dirname "$0")" || exit 2
python3 scenarios.py > results.txt; s=$?
python3 simulate.py >> results.txt; t=$?
cat results.txt
exit $(( s + t ))
