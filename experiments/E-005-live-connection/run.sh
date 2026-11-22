#!/usr/bin/env bash
# Reruns E-005 in a container with a real (headless) browser and no outside network. About two minutes.
set -euo pipefail
cd "$(dirname "$0")"
docker image inspect contexia-browser:1.49.0 >/dev/null 2>&1 || docker build -q -t contexia-browser:1.49.0 ../../tools/browser >/dev/null
docker build -q -t contexia-e005 . >/dev/null
exec docker run --rm --network none --ipc=host -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$PWD":/exp -w /exp contexia-e005 python3 -u test.py
