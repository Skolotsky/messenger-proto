#!/usr/bin/env bash
# Reruns E-004: starts PostgreSQL 17 in Docker, runs the tests against it, removes it. About three minutes.
set -euo pipefail
cd "$(dirname "$0")"
docker build -q -t contexia-e004 . >/dev/null
docker network create e004-net >/dev/null 2>&1 || true
docker rm -f e004-pg >/dev/null 2>&1 || true
docker run -d --name e004-pg --network e004-net -e POSTGRES_PASSWORD=e004 postgres:17-alpine >/dev/null
trap 'docker rm -f e004-pg >/dev/null 2>&1; docker network rm e004-net >/dev/null 2>&1' EXIT
docker run --rm --network e004-net -u "$(id -u):$(id -g)" -v "$PWD":/exp contexia-e004 sh -c "${E004_ONLY:-python3 -u test.py && python3 -u test_teams.py}"
