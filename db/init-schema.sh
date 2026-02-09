#!/bin/bash
# Automatic schema initialization script for ScyllaDB

echo "🔄 Waiting for ScyllaDB to be ready..."

# ScyllaDB host
SCYLLA_HOST="${SCYLLA_HOST:-scheduler-scylla}"

# Wait for ScyllaDB to be fully operational
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if cqlsh $SCYLLA_HOST -e "SELECT now() FROM system.local;" > /dev/null 2>&1; then
        echo "✅ ScyllaDB is ready!"
        break
    fi
    
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "⏳ Attempt $RETRY_COUNT/$MAX_RETRIES - ScyllaDB not ready yet, waiting..."
    sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "❌ Failed to connect to ScyllaDB after $MAX_RETRIES attempts"
    exit 1
fi

# Check if schema already exists
if cqlsh $SCYLLA_HOST -e "USE scheduler;" > /dev/null 2>&1; then
    echo "ℹ️  Schema already exists, skipping initialization"
    exit 0
fi

# Initialize schema
echo "🔧 Initializing schema from /opt/schema.cql..."
if cqlsh $SCYLLA_HOST -f /opt/schema.cql; then
    echo "✅ Schema initialized successfully!"
else
    echo "❌ Failed to initialize schema"
    exit 1
fi
