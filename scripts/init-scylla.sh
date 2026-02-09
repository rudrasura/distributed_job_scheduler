#!/bin/bash
set -e

SCYLLA_HOST="scheduler-scylla"
MAX_RETRIES=30
RETRY_INTERVAL=2

echo "Waiting for ScyllaDB to be ready at $SCYLLA_HOST..."

for i in $(seq 1 $MAX_RETRIES); do
  if docker exec $SCYLLA_HOST cqlsh -e "DESCRIBE KEYSPACES" > /dev/null 2>&1; then
    echo "ScyllaDB is ready!"
    break
  fi
  echo "Attempt $i/$MAX_RETRIES: ScyllaDB not ready yet. Retrying in $RETRY_INTERVAL seconds..."
  sleep $RETRY_INTERVAL
done

if [ $i -eq $MAX_RETRIES ]; then
  echo "Error: ScyllaDB failed to start within the timeout period."
  exit 1
fi

echo "Initializing Schema..."
docker cp db/schema.cql scheduler-scylla:/tmp/schema.cql
docker exec scheduler-scylla cqlsh -f /tmp/schema.cql

echo "Schema initialized successfully."
