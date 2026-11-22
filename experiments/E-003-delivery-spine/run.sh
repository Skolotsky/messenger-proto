#!/usr/bin/env bash
# Reruns E-003. About nine minutes.
# Standard library only; nothing is installed or downloaded.
# The full-size rebuild is separate (about five minutes): BATCH=1 E003_PEOPLE=36 E003_CHANNELS=20 python3 run.py rebuild-full
cd "$(dirname "$0")" && exec python3 run.py "$@"
