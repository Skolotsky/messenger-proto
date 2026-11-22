#!/usr/bin/env bash
# Reruns E-006: PostgreSQL 17 and Apache Kafka 3.9 (one node) in Docker, a relay and two receivers between them,
# faults injected for real (processes killed, Kafka stopped). About six minutes. Removes its containers afterwards.
set -euo pipefail
cd "$(dirname "$0")"
docker build -q -t contexia-e006 . >/dev/null
docker network create e006-net >/dev/null 2>&1 || true
docker rm -f e006-pg e006-kafka >/dev/null 2>&1 || true
docker run -d --name e006-pg --network e006-net -e POSTGRES_PASSWORD=e006 postgres:17-alpine >/dev/null
docker run -d --name e006-kafka --network e006-net \
  -e KAFKA_NODE_ID=1 -e KAFKA_PROCESS_ROLES=broker,controller \
  -e KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://e006-kafka:9092 \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
  -e KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093 -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  -e KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0 apache/kafka:3.9.0 >/dev/null
rm -f .ctl .ack
# The test asks for Kafka to be stopped and started by writing a word into .ctl; this loop does it.
( while true; do
    if [ -s .ctl ]; then cmd=$(cat .ctl); : > .ctl; docker "$cmd" e006-kafka >/dev/null 2>&1; echo "$cmd" > .ack; fi
    sleep 0.2
  done ) &
watcher=$!
trap 'kill $watcher 2>/dev/null; docker rm -f e006-pg e006-kafka >/dev/null 2>&1; docker network rm e006-net >/dev/null 2>&1; rm -f .ctl .ack' EXIT
docker run --rm --network e006-net -u "$(id -u):$(id -g)" -v "$PWD":/exp contexia-e006 python3 -u test.py "$@"
