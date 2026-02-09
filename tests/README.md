# Test Suite Quick Reference

## Integration Tests (`tests/integration`)

### TestImmediateJobExecution
**Duration:** ~7.5s  
**Purpose:** Validates immediate job submission and execution  
**Verification:** Job completes within expected time
```bash
go test -v ./tests/integration -run TestImmediateJobExecution
```

### TestScheduledJobExecution
**Duration:** ~5.5s  
**Purpose:** Verifies jobs scheduled for future execution  
**Verification:** Job executes at specified time
```bash
go test -v ./tests/integration -run TestScheduledJobExecution
```

### TestRecurringJobExecution  
**Duration:** ~5s  
**Purpose:** Validates cron-based recurring jobs  
**Verification:** Multiple runs with correct intervals
```bash
go test -v ./tests/integration -run TestRecurringJobExecution
```

### TestLoad
**Duration:** ~8.1s  
**Purpose:** Load test with 100 concurrent job submissions  
**Metrics:** 1,064 jobs/sec submission, 12.34 jobs/sec end-to-end throughput  
**Verification:** All jobs complete successfully
```bash
go test -v ./tests/integration -run TestLoad
```

### TestS3OffloadingAndMetrics
**Duration:** ~2.5s  
**Purpose:** Validates S3 payload offloading for large payloads (>1KB)  
**Verification:** Payload uploaded to S3, downloaded by worker, executed correctly
```bash
go test -v ./tests/integration -run TestS3OffloadingAndMetrics
```

### TestSQSRestart
**Duration:** ~32s  
**Purpose:** Verify system resilience when SQS is restarted  
**Verification:** Jobs continue processing after SQS restart
```bash
go test -v ./tests/integration -run TestSQSRestart
```

---

## Chaos Tests (`tests/chaos`)

### TestPickerFailure
**Duration:** ~18s  
**Purpose:** Test system recovery when Picker service crashes  
**Scenario:**
- Submit scheduled job
- Kill Picker before execution time
- Verify job doesn't execute while Picker is down
- Restart Picker
- Verify job executes after recovery

```bash
go test -v ./tests/chaos -run TestPickerFailure
```

### TestWorkerFailure
**Duration:** ~62s  
**Purpose:** Test system recovery when Worker service crashes mid-execution  
**Scenario:**
- Submit long-running job (30s sleep)
- Kill Worker during execution
- Restart Worker
- Verify job is reprocessed from SQS

```bash
go test -v ./tests/chaos -run TestWorkerFailure
```

---

## Run All Tests

```bash
# All integration tests
go test -v ./tests/integration -timeout 2m

# All chaos tests
go test -v ./tests/chaos -timeout 3m

# Full test suite
go test -v ./tests/... -timeout 5m
```

---

## Test Coverage Summary

### ✅ Covered Scenarios (8 tests)
- Immediate job execution
- Scheduled job execution  
- Recurring jobs (cron-based)
- Large payload S3 offloading (>1KB)
- High throughput load (100+ concurrent jobs)
- SQS resilience (restart recovery)
- Picker service failure recovery
- Worker service failure recovery

### ⏭️ Future Tests
- ScyllaDB failure recovery
- Kafka restart recovery
- Redis failure scenarios
- Etcd leader election failover
- Network partition scenarios
- Concurrent worker scaling
- Job retry mechanism verification
- Dead letter queue handling
