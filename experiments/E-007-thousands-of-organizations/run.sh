#!/usr/bin/env bash
# Reruns E-007: PostgreSQL 17 in Docker with 5,000 organizations, writers and delivery workers as separate
# processes. About six minutes. Uses the client image of E-004 (Python with the PostgreSQL driver).
set -euo pipefail
cd "$(dirname "$0")"
docker build -q -t contexia-e004 ../E-004-postgres-log >/dev/null
docker network create e007-net >/dev/null 2>&1 || true
docker rm -f e007-pg >/dev/null 2>&1 || true
docker run -d --name e007-pg --network e007-net -e POSTGRES_PASSWORD=e007 --shm-size=1g postgres:17-alpine -c max_connections=300 >/dev/null
trap 'docker rm -f e007-pg >/dev/null 2>&1; docker network rm e007-net >/dev/null 2>&1' EXIT
docker run --rm --network e007-net -u "$(id -u):$(id -g)" -v "$PWD":/exp -w /exp contexia-e004 python3 -u "${E007_TEST:-test.py}" "$@"
