# Distributed Job Scheduler - End-to-End Demo

This guide walks you through a complete demonstration of the distributed job scheduler, from startup to advanced features like S3 payload offloading and chaos testing.

---

## Prerequisites

- Docker & Docker Compose installed
- Go 1.21+ (for running tests)
- curl (for API interactions)

---

## Part 1: System Startup

### 1.1 Start All Services

```bash
cd /Users/mukeshpai/work/distributed_job_scheduler

# Start infrastructure and application services
docker-compose up -d

# Verify all containers are running
docker-compose ps
```

**Expected Output:**
```
NAME                     STATUS
scheduler-coordinator    Up
scheduler-etcd           Up
scheduler-grafana        Up
scheduler-ingestion      Up
scheduler-kafka          Up
scheduler-picker         Up
scheduler-prometheus     Up
scheduler-redis          Up
scheduler-s3             Up
scheduler-scylla         Up
scheduler-sqs            Up
scheduler-worker         Up
scheduler-writer         Up
scheduler-zookeeper      Up
```

### 1.2 Verify Schema Initialization

The schema is **automatically initialized** by the `schema-init` container:

```bash
# Check initialization logs
docker logs scheduler-schema-init

# Expected output:
# 🔄 Waiting for ScyllaDB to be ready...
# ✅ ScyllaDB is ready!
# 🔧 Initializing schema from /opt/schema.cql...
# ✅ Schema initialized successfully!
```

**Verify Schema:**
```bash
docker exec -it scheduler-scylla cqlsh scheduler-scylla -e "USE scheduler; DESCRIBE TABLES;"
```

**Expected Output:**
```
job_queue  job_runs  jobs
```

---

## Part 2: Job Submission & Execution

### 2.1 Submit an Immediate Job

```bash
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d '{
    "project_id": "demo-project",
    "payload": "Hello from immediate job!",
    "max_retries": 3
  }'
```

**Expected Response:**
```json
{
  "job_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "Submitted",
  "message": "Job submitted successfully"
}
```

**Verify Execution:**
```bash
# Watch container logs
docker logs -f scheduler-worker

# Expected log output:
# Executing Job a1b2c3d4-e5f6-7890-abcd-ef1234567890 (Run ...) Hello from immediate job!
# Job a1b2c3d4-e5f6-7890-abcd-ef1234567890 Completed
```

### 2.2 Submit a Scheduled Job

```bash
# Schedule job for 30 seconds from now
FIRE_TIME=$(date -u -v+30S +"%Y-%m-%dT%H:%M:%SZ")

curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d "{
    \"project_id\": \"demo-project\",
    \"payload\": \"Scheduled job payload\",
    \"next_fire_at\": \"$FIRE_TIME\",
    \"max_retries\": 3
  }"
```

**Verify Delayed Execution:**
```bash
# Job won't execute immediately
# Wait 30+ seconds, then check logs
docker logs scheduler-worker --tail 10
```

### 2.3 Submit a Recurring Job

```bash
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d '{
    "project_id": "demo-project",
    "payload": "Recurring task",
    "cron_schedule": "*/2 * * * * *",
    "max_retries": 3
  }'
```

**Observe Recurring Execution:**
```bash
# Watch as job executes every 2 seconds
docker logs -f scheduler-worker | grep "Recurring task"

# See rescheduling logs
docker logs -f scheduler-worker | grep "Rescheduling"
```

### 2.4 Execute Shell Commands (curl Example)

The worker can execute shell commands using the `cmd:` prefix. This example demonstrates executing a curl command to fetch a webpage:

```bash
# Execute a curl command to fetch example.com
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d '{
    "project_id": "command-demo",
    "payload": "cmd:curl -s https://www.example.com/",
    "max_retries": 3
  }'
```

**Expected Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "Submitted",
  "message": "Job submitted successfully"
}
```

**Step-by-Step Verification:**

**1. Monitor Worker Execution:**
```bash
# Watch worker logs in real-time
docker logs -f scheduler-worker

# Expected output:
# Executing command: curl -s https://www.example.com/
# Command executed successfully. Output length: 1256 bytes
# Job <job_id> Completed
```

**2. Query Job Run from Database:**
```bash
# Get the job_id from the submit response, then query
docker exec -it scheduler-scylla cqlsh scheduler-scylla -e "
  SELECT job_id, status, LEFT(output, 100) as output_preview 
  FROM scheduler.job_runs 
  WHERE job_id = <job_id> 
  ALLOW FILTERING;
"
```

**Expected Output:**
```
 job_id                               | status    | output_preview
--------------------------------------+-----------+----------------------------------
 550e8400-e29b-41d4-a716-446655440000 | COMPLETED | <!doctype html><html><head><title>Example Domain</title>...
```

**3. Verify Full Output Contains HTML:**
```bash
# Query full output to verify curl succeeded
docker exec -it scheduler-scylla cqlsh scheduler-scylla -e "
  SELECT output 
  FROM scheduler.job_runs 
  WHERE job_id = <job_id> 
  ALLOW FILTERING;
" | grep -i "Example Domain"
```

**Expected:** Output contains "Example Domain" and valid HTML tags.


### 2.5 Execute Large Shell Scripts (>1KB with S3 Offloading)

For payloads larger than 1KB, the system **automatically offloads to S3**. This example demonstrates a comprehensive multi-task shell script:

**Step 1: Create a Large Shell Script (>1KB)**

```bash
# Create a comprehensive shell script with multiple operations
cat > /tmp/large_test_script.sh <<'SCRIPT'
#!/bin/bash
echo "======================================"
echo "=== Multi-Task Shell Script Demo ==="
echo "======================================"
echo ""

# System Information
echo "--- System Information ---"
echo "Hostname: $(hostname)"
echo "Current Time: $(date)"
echo "Current User: $(whoami)"
echo "Working Directory: $(pwd)"
echo ""

# Create temporary directory and files
echo "--- File Operations ---"
TMP_DIR="/tmp/script_test_$$"
mkdir -p "$TMP_DIR"
echo "Created directory: $TMP_DIR"

for i in {1..10}; do
    echo "Test file content for iteration $i" > "$TMP_DIR/file_$i.txt"
done
FILE_COUNT=$(ls -1 "$TMP_DIR" | wc -l)
echo "Created $FILE_COUNT test files"
echo ""

# Mathematical calculations
echo "--- Mathematical Operations ---"
SUM=0
for i in {1..100}; do 
    SUM=$((SUM + i))
done
echo "Sum of numbers 1-100: $SUM (Expected: 5050)"
echo ""

# String operations
echo "--- String Operations ---"
TEST_STRING="Distributed Job Scheduler"
echo "Original: $TEST_STRING"
echo "Uppercase: $(echo $TEST_STRING | tr '[:lower:]' '[:upper:]')"
echo "Length: ${#TEST_STRING} characters"
echo ""

# Network check
echo "--- Network Check ---"
if curl -s -o /dev/null -w "%{http_code}" https://www.example.com/ | grep -q "200"; then
    echo "✓ Network connectivity: OK"
else
    echo "✗ Network connectivity: FAILED"
fi
echo ""

# Cleanup
echo "--- Cleanup ---"
rm -rf "$TMP_DIR"
echo "Removed temporary directory"
echo ""

echo "======================================"
echo "=== Script Execution Complete ✓ ==="
echo "======================================"
SCRIPT

# Check script size
echo "Script size: $(wc -c < /tmp/large_test_script.sh) bytes"
```

**Step 2: Base64 Encode and Submit**

```bash
# Base64 encode the script for safe transmission
SCRIPT_B64=$(base64 < /tmp/large_test_script.sh)

# Submit the job
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d "{
    \"project_id\": \"large-script-demo\",
    \"payload\": \"cmd:echo '$SCRIPT_B64' | base64 -d | bash\",
    \"max_retries\": 3
  }"
```

**Expected Response:**
```json
{
  "job_id": "7a8b9c0d-1e2f-3a4b-5c6d-7e8f9a0b1c2d",
  "status": "Submitted",
  "message": "Job submitted successfully"
}
```

**Step 3: Verify S3 Offloading**

Since the payload is >1KB, it should be automatically offloaded to S3:

```bash
# Check ingestion service logs for S3 upload
docker logs scheduler-ingestion --tail 30 | grep -i "s3"

# Expected output:
# Payload size 2418 bytes exceeds 1KB, uploading to S3...
# Successfully uploaded to S3: s3://scheduler-payloads/<job_id>
```

**Step 4: Monitor Prometheus Metrics**

```bash
# Query S3 upload metrics
curl -s http://localhost:9090/api/v1/query?query=s3_operations_total | jq

# Expected: s3_operations_total{operation="upload"} counter incremented
```

**Step 5: Verify Script Execution**

```bash
# Watch worker logs for execution
docker logs -f scheduler-worker

# Expected output includes:
# Executing command: echo '...' | base64 -d | bash
# === Multi-Task Shell Script Demo ===
# Sum of numbers 1-100: 5050 (Expected: 5050)
# ✓ Network connectivity: OK
# === Script Execution Complete ✓ ===
# Command executed successfully. Output length: 1547 bytes
```

**Step 6: Query Job Results**

```bash
# Query job run with script output
docker exec -it scheduler-scylla cqlsh scheduler-scylla -e "
  SELECT job_id, status, triggered_at, completed_at 
  FROM scheduler.job_runs 
  WHERE job_id = <job_id> 
  ALLOW FILTERING;
"

# View script output
docker exec -it scheduler-scylla cqlsh scheduler-scylla -e "
  SELECT output 
  FROM scheduler.job_runs 
  WHERE job_id = <job_id> 
  ALLOW FILTERING;
" | grep "Script Execution Complete"
```

**Expected Results:**
- ✅ Status: COMPLETED
- ✅ Output contains: "Sum of numbers 1-100: 5050"
- ✅ Output contains: "Script Execution Complete ✓"
- ✅ Payload was offloaded to S3 (check ingestion logs)
- ✅ S3 metrics incremented


### 2.6 Recurring Shell Script Jobs

Combine shell scripts with cron scheduling:

```bash
# Create monitoring script
MONITOR_SCRIPT=$(cat <<'EOF' | base64
#!/bin/bash
echo "=== System Monitor ==="
echo "Timestamp: $(date +%s)"
echo "Hostname: $(hostname)"
RANDOM=$((RANDOM % 1000))
echo "Random metric: $RANDOM"
echo "MONITOR_RUN_COMPLETE"
EOF
)

# Submit as recurring job (every 5 seconds)
FIRE_TIME=$(date -u -v+5S +"%Y-%m-%dT%H:%M:%SZ")

curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d "{
    \"project_id\": \"recurring-monitor\",
    \"payload\": \"cmd:echo '$MONITOR_SCRIPT' | base64 -d | bash\",
    \"cron_schedule\": \"*/5 * * * * *\",
    \"next_fire_at\": \"$FIRE_TIME\",
    \"max_retries\": 3
  }"
```

**Verify Recurring Execution:**
```bash
# Watch for multiple executions
docker logs -f scheduler-worker | grep "MONITOR_RUN_COMPLETE"

# Expected: Output every 5 seconds

# Query all runs for the job
docker exec -it scheduler-scylla cqlsh -e "
  SELECT run_id, triggered_at, status 
  FROM scheduler.job_runs 
  WHERE job_id = <job_id> 
  ALLOW FILTERING;
"

# Verify job is rescheduled
docker exec -it scheduler-scylla cqlsh -e "
  SELECT job_id, next_fire_at, cron_schedule 
  FROM scheduler.jobs 
  WHERE job_id = <job_id>;
"
```

### 2.7 Submit a Large Payload (S3 Offloading)

```bash
# Create payload >1KB to trigger S3 offloading
LARGE_PAYLOAD=$(python3 -c "print('X' * 2048)")

curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d "{
    \"project_id\": \"s3-demo\",
    \"payload\": \"$LARGE_PAYLOAD\",
    \"max_retries\": 3
  }"
```

**Verify S3 Offloading:**
```bash
# Check ingestion logs for S3 upload
docker logs scheduler-ingestion --tail 20 | grep "s3"

# Check worker logs for S3 download
docker logs scheduler-worker --tail 20 | grep "s3"
```

---

## Part 3: Query Jobs

### 3.1 Get Job by ID

```bash
JOB_ID="<job_id_from_submit_response>"

curl -X GET "http://localhost:8080/job?id=$JOB_ID"
```

**Expected Response:**
```json
{
  "job_id": "a1b2c3d4-...",
  "project_id": "demo-project",
  "payload": "Hello from immediate job!",
  "status": "COMPLETED",
  "next_fire_at": "2026-02-09T...",
  "created_at": "2026-02-09T..."
}
```

### 3.2 Get All Jobs for a User

```bash
curl -X GET http://localhost:8080/jobs \
  -H "X-User-ID: demo-user"
```

**Expected Response:**
```json
[
  {
    "job_id": "...",
    "status": "COMPLETED",
    "next_fire_at": "...",
    "created_at": "..."
  },
  ...
]
```

---

## Part 4: Metrics & Observability

### 4.1 Access Prometheus

```bash
# Open in browser
open http://localhost:9090
```

**Sample Queries:**
1. **HTTP Request Rate:** `rate(http_requests_total[1m])`
2. **Job Execution Rate:** `rate(jobs_executed_total[1m])`
3. **S3 Operations:** `s3_operations_total`
4. **Job Execution Duration (p99):** `histogram_quantile(0.99, job_execution_duration_seconds_bucket)`

### 4.2 Access Grafana Dashboard

```bash
# Open in browser
open http://localhost:3000

# Login: admin / admin
```

**Navigate to Dashboard:**
1. Go to "Dashboards" → "Job Scheduler Overview"
2. View real-time metrics:
   - Ingestion Throughput
   - Job Execution Rate
   - Success/Failure Trends

### 4.3 Direct Metrics Access

```bash
# Ingestion Service Metrics
curl http://localhost:8081/metrics | grep http_requests_total

# Picker Service Metrics
curl http://localhost:8082/metrics | grep jobs_scanned_total

# Worker Service Metrics
curl http://localhost:8083/metrics | grep jobs_executed_total
```

---

## Part 5: Running Tests

### 5.1 Integration Tests

```bash
# Run all integration tests
go test -v ./tests/integration -timeout 2m

# Run specific tests
go test -v ./tests/integration -run TestImmediateJobExecution
go test -v ./tests/integration -run TestRecurringJobExecution
go test -v ./tests/integration -run TestS3OffloadingAndMetrics
go test -v ./tests/integration -run TestLoad
```

**Expected Results:**
```
✓ TestCommandExecution          (1.5s)
✓ TestImmediateJobExecution     (2.5s)
✓ TestLoad                      (6.1s) - 16.30 jobs/sec
✓ TestRecurringJobExecution     (5s)
✓ TestRecurringShellScript      (10s) - 3 executions verified
✓ TestS3OffloadingAndMetrics    (0.5s)
✓ TestScheduledJobExecution     (6s)
✓ TestShellScriptExecution      (2.5s) - 2418 byte payload
✓ TestSQSRestart               (15s)
```

### 5.2 Chaos Tests

```bash
# Run all chaos tests
go test -v ./tests/chaos -timeout 3m

# Run specific chaos tests
go test -v ./tests/chaos -run TestPickerFailure
go test -v ./tests/chaos -run TestWorkerFailure
```

**Expected Results:**
```
✓ TestPickerFailure   (18s)  - Job recovered after Picker restart
✓ TestWorkerFailure   (62s)  - Job recovered after Worker restart
```

---

## Part 6: Chaos Engineering Demo

### 6.1 Kill Picker (Manual Demo)

```bash
# 1. Submit a scheduled job
FIRE_TIME=$(date -u -v+20S +"%Y-%m-%dT%H:%M:%SZ")
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: chaos-demo" \
  -d "{
    \"project_id\": \"chaos-test\",
    \"payload\": \"Picker resilience test\",
    \"next_fire_at\": \"$FIRE_TIME\"
  }"

# 2. Kill the Picker before execution
docker kill scheduler-picker

# 3. Wait past the fire time (job won't execute)
sleep 25

# 4. Restart Picker
docker-compose up -d picker-service

# 5. Observe recovery - job should execute shortly
docker logs -f scheduler-worker
```

### 6.2 Kill Worker During Execution

```bash
# 1. Submit a long-running job
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: chaos-demo" \
  -d '{
    "project_id": "chaos-test",
    "payload": "sleep:30s",
    "max_retries": 3
  }'

# 2. Verify job started
docker logs scheduler-worker --tail 5

# 3. Kill worker mid-execution
docker kill scheduler-worker

# 4. Restart worker
docker-compose up -d worker-service

# 5. Job will be reprocessed from SQS
docker logs -f scheduler-worker
```

### 6.3 Restart SQS

```bash
# 1. Submit multiple jobs
for i in {1..5}; do
  curl -X POST http://localhost:8080/submit \
    -H "Content-Type: application/json" \
    -H "X-User-ID: sqs-demo" \
    -d "{
      \"project_id\": \"sqs-resilience\",
      \"payload\": \"Job $i\"
    }"
done

# 2. Restart SQS
docker restart scheduler-sqs

# 3. Verify all jobs still execute
docker logs -f scheduler-worker
```

---

## Part 7: Advanced Scenarios

### 7.1 Load Testing

```bash
# Submit 100 jobs concurrently
go test -v ./tests/integration -run TestLoad

# Observe metrics in Grafana
# - Throughput spike
# - Latency distribution
# - Queue depth
```

### 7.2 Verify Task Execution

```bash
# Submit a job that produces clear output
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: execution-demo" \
  -d '{
    "project_id": "verify-execution",
    "payload": "VERIFICATION_PAYLOAD_12345",
    "max_retries": 3
  }'

# Check worker logs for execution proof
docker logs scheduler-worker 2>&1 | grep "VERIFICATION_PAYLOAD_12345"

# Expected:
# Executing Job ... VERIFICATION_PAYLOAD_12345
# Job ... Completed
```

### 7.3 Query Job Runs (ScyllaDB)

```bash
# Query job_runs table directly
docker exec -it scheduler-scylla cqlsh -e "
  SELECT job_id, run_id, status, output
  FROM scheduler.job_runs
  WHERE job_id = <job_uuid>
  ALLOW FILTERING;
"
```

---

## Part 8: Cleanup

```bash
# Stop all services
docker-compose down

# Remove volumes (complete reset)
docker-compose down -v

# Remove only application data
docker volume rm distributed_job_scheduler_scylla-data
docker volume rm distributed_job_scheduler_redis-data
```

---

## Demo Checklist

Use this checklist for a complete demo presentation:

- [ ] **System Startup**
  - [ ] All 14 containers running
  - [ ] Scylla schema initialized
  - [ ] API health check responds

- [ ] **Basic Job Execution**
  - [ ] Immediate job executes within 5s
  - [ ] Scheduled job executes at specified time
  - [ ] Recurring job executes multiple times

- [ ] **S3 Offloading**
  - [ ] Submit >1KB payload
  - [ ] Verify S3 upload in ingestion logs
  - [ ] Verify S3 download in worker logs
  - [ ] Job completes with full payload

- [ ] **Metrics & Dashboards**
  - [ ] Prometheus shows `http_requests_total`
  - [ ] Grafana dashboard displays data
  - [ ] All 3 metrics endpoints accessible (8081, 8082, 8083)

- [ ] **API Functionality**
  - [ ] Query job by ID
  - [ ] Query all jobs for user
  - [ ] Correct job status updates

- [ ] **Tests**
  - [ ] All 9 integration tests pass
  - [ ] Both chaos tests pass
  - [ ] Load test achieves >10 jobs/sec
  - [ ] Shell script execution works
  - [ ] Recurring shell scripts execute multiple times

- [ ] **Chaos Engineering**
  - [ ] Picker failure recovery
  - [ ] Worker failure recovery
  - [ ] SQS restart resilience

---

## Troubleshooting

### Services Not Starting

```bash
# Check logs for specific service
docker logs scheduler-<service-name>

# Common issues:
# - Scylla: Wait 60s for initialization
# - Kafka: Depends on Zookeeper (wait 10s)
# - Picker/Worker: Need Scylla schema
```

### Jobs Not Executing

```bash
# 1. Verify Writer is consuming from Kafka
docker logs scheduler-writer

# 2. Check job_queue has entries
docker exec -it scheduler-scylla cqlsh -e "
  SELECT COUNT(*) FROM scheduler.job_queue;
"

# 3. Verify Picker is scanning
docker logs scheduler-picker

# 4. Check SQS has messages
curl http://localhost:9325/queues/scheduler-job-queue
```

### Metrics Not Showing

```bash
# Verify service metrics ports
curl http://localhost:8081/metrics  # Ingestion
curl http://localhost:8082/metrics  # Picker
curl http://localhost:8083/metrics  # Worker

# Check Prometheus targets
# http://localhost:9090/targets
# All should be "UP"
```

---

## Summary

This demo showcases:

1. ✅ **Immediate, Scheduled, and Recurring Jobs**
2. ✅ **S3 Payload Offloading** (>1KB)
3. ✅ **Prometheus Metrics** (HTTP, Jobs, S3, SQS)
4. ✅ **Grafana Dashboards**
5. ✅ **Comprehensive Test Suite** (Integration + Chaos)
6. ✅ **Fault Tolerance** (Picker, Worker, SQS failures)
7. ✅ **High Throughput** (12+ jobs/sec)
8. ✅ **ScyllaDB + Redis + Kafka + SQS Architecture**

**Next Steps:**
- Explore remaining ScyllaDB/Kafka chaos tests
- Implement Helm charts for production deployment
- Add authentication/authorization layer
