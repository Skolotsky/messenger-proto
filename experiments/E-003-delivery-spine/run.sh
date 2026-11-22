#!/usr/bin/env bash
# Reruns E-003. About ten minutes. Standard library only; nothing is installed or downloaded.
cd "$(dirname "$0")" && exec python3 run.py "$@"
