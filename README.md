# Distributed Job Scheduler

A production-grade, horizontally scalable distributed job scheduling system built with Go, designed for high availability, fault tolerance, and operational excellence.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-11/11_Passing-success)](./TEST_RESULTS.md)

## 📖 Documentation

**[📘 Complete System Documentation (Notion)](https://www.notion.so/Job-Scheduler-2bf98102139b80bb85fde3c663647f80?source=copy_link)**

For detailed architecture, design decisions, and implementation details, refer to the comprehensive Notion documentation above.

---

## 🎯 Overview

A distributed job scheduler that supports immediate, scheduled, and recurring job execution with advanced features like:
- **Shell command/script execution** with `cmd:` prefix
- **S3 payload offloading** for large job payloads (>1KB)
- **SQS-based job queuing** for reliable message delivery
- **Fault-tolerant architecture** with automatic recovery
- **Comprehensive observability** with Prometheus & Grafana

### System Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Ingestion  │─────▶│   Writer    │─────▶│   Picker    │
│   Service   │      │   Service   │      │   Service   │
└─────────────┘      └─────────────┘      └─────────────┘
       │                     │                     │
       │                     ▼                     ▼
       │              ┌─────────────┐      ┌─────────────┐
       └─────────────▶│   Scylla    │      │     SQS     │
                      │     DB      │      │   Queue     │
                      └─────────────┘      └─────────────┘
                                                  │
                                                  ▼
                                           ┌─────────────┐
                                           │   Worker    │
                                           │   Service   │
                                           └─────────────┘
```

---

## ✨ Key Features

### Job Types
- **Immediate Jobs** - Execute as soon as resources are available
- **Scheduled Jobs** - Execute at a specific future time (RFC3339 format)
- **Recurring Jobs** - Execute on a cron schedule (supports second-level granularity)

### Advanced Capabilities
- ✅ **Shell Script Execution** - Execute shell commands and scripts using `cmd:` prefix
- ✅ **S3 Payload Offloading** - Automatically offload payloads >1KB to S3
- ✅ **SQS Message Queue** - Reliable job delivery with ElasticMQ
- ✅ **Leader Election** - Coordinator service with Etcd-based leader election
- ✅ **Horizontal Scalability** - Multiple workers, pickers with shard-based distribution
- ✅ **Fault Tolerance** - Automatic recovery from worker/picker failures
- ✅ **Observability** - Prometheus metrics + Grafana dashboards
- ✅ **Chaos Engineering** - Verified resilience through chaos tests

---

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.22+ (for running tests)
- `curl` (for API interactions)

### 1. Start the System

```bash
# Clone the repository
git clone <repository-url>
cd distributed_job_scheduler

# Start all services ( 14 containers)
docker-compose up -d

# Verify all containers are running
docker-compose ps
```

### 2. Initialize Database Schema

The schema is **automatically initialized** when you start the services! A `schema-init` sidecar container will:
- Wait for ScyllaDB to be ready
- Check if the schema exists
- Initialize the schema if needed

```bash
# Verify schema was initialized successfully
docker logs scheduler-schema-init

# Expected output:
# ✅ ScyllaDB is ready!
# 🔧 Initializing schema from /opt/schema.cql...
# ✅ Schema initialized successfully!
```

**Manual initialization (if needed):**
```bash
docker exec -it scheduler-scylla cqlsh scheduler-scylla -f /opt/schema.cql
```

### 3. Submit Your First Job

```bash
# Submit an immediate job
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d '{
    "project_id": "demo-project",
    "payload": "Hello, Distributed Scheduler!",
    "max_retries": 3
  }'
```

### 4. Execute a Shell Command

```bash
# Execute a curl command
curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: demo-user" \
  -d '{
    "project_id": "shell-demo",
    "payload": "cmd:curl -s https://www.example.com/",
    "max_retries": 3
  }'
```

---

## 📡 API Reference

### Base URL
`http://localhost:8080`

### Submit Job
**POST** `/submit`

**Headers:**
- `X-User-ID: <user-id>` (Required)
- `Content-Type: application/json`

**Request Body:**
```json
{
  "project_id": "my-project",
  "payload": "job-payload-or-cmd:shell-command",
  "cron_schedule": "@every 5m",
  "next_fire_at": "2026-12-31T23:59:59Z",
  "max_retries": 3
}
```

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "Submitted",
  "message": "Job submitted successfully"
}
```

### Get Job Details
**GET** `/job?id=<job_id>`

### List User Jobs
**GET** `/jobs`
- Headers: `X-User-ID: <user-id>` (Required)

---

## 🧪 Testing

### Integration Tests (9 tests)
```bash
# Run all integration tests
go test -v ./tests/integration -timeout 90s

# Run specific test
go test -v ./tests/integration -run TestShellScriptExecution
```

**Test Suite:**
- ✅ TestCommandExecution (1.5s)
- ✅ TestImmediateJobExecution (2.5s)
- ✅ TestLoad (8.1s) - 12.33 jobs/sec
- ✅ TestRecurringJobExecution (6.0s)
- ✅ TestRecurringShellScript (10s) - 4 runs verified
- ✅ TestS3OffloadingAndMetrics (3.0s)
- ✅ TestScheduledJobExecution (6.0s)
- ✅ TestShellScriptExecution (2.5s) - 2418 byte payload
- ✅ TestSQSRestart (13s)

### Chaos Tests (2 tests)
```bash
# Run chaos/resilience tests
go test -v ./tests/chaos -timeout 3m
```

**Chaos Suite:**
- ✅ TestPickerFailure (~18s) - Automatic recovery
- ✅ TestWorkerFailure (~62s) - Job reprocessing from SQS

**📊 See [TEST_RESULTS.md](./TEST_RESULTS.md) for detailed test results**

---

## 🏗️ System Components

| Component | Purpose | Port | Technology |
|-----------|---------|------|------------|
| **Ingestion** | HTTP API for job submission | 8080 | Go + HTTP |
| **Writer** | Kafka consumer, writes to Scylla | - | Go + Kafka |
| **Coordinator** | Leader election for Picker | - | Go + Etcd |
| **Picker** | Scans job_queue, publishes to SQS | - | Go + Etcd |
| **Worker** | Executes jobs from SQS | - | Go + SQS |
| **Scylla** | Primary database (jobs, job_runs) | 9042 | ScyllaDB |
| **Redis** | Distributed locks (future) | 6379 | Redis 7.0 |
| **Kafka** | Event streaming | 29092 | Kafka 3.5 |
| **SQS** | Job queue | 9324 | ElasticMQ |
| **S3** | Large payload storage | 4566 | LocalStack |
| **Etcd** | Distributed coordination | 2379 | Etcd v3.5 |
| **Prometheus** | Metrics collection | 9090 | Prometheus |
| **Grafana** | Metrics visualization | 3000 | Grafana |

---

## 📊 Observability

### Prometheus Metrics
Access: `http://localhost:9090`

**Key Metrics:**
- `http_requests_total` - API request count
- `jobs_executed_total{status="success|failed"}` - Job execution status
- `job_execution_duration_seconds` - Job execution latency
- `s3_operations_total{operation="upload|download"}` - S3 operations
- `sqs_enqueue_duration_seconds` - SQS publish latency

### Grafana Dashboards
Access: `http://localhost:3000` (admin/admin)

**Pre-configured Dashboards:**
- Job Scheduler Overview
- System Performance Metrics
- Error Rates & Latencies

### Service Metrics Endpoints
- Ingestion: `http://localhost:8081/metrics`
- Picker: `http://localhost:8082/metrics`
- Worker: `http://localhost:8083/metrics`

---

## 🎬 Demo Guide

For a complete end-to-end demonstration including:
- Shell script execution (>1KB with S3 offloading)
- Recurring shell script jobs
- Chaos engineering scenarios
- Metrics visualization

**📘 See [DEMO.md](./DEMO.md) for step-by-step instructions**

---

## 🗄️ Database Schema

### `jobs` Table
```cql
CREATE TABLE scheduler.jobs (
    job_id uuid PRIMARY KEY,
    user_id text,
    project_id text,
    payload text,
    cron_schedule text,
    next_fire_at timestamp,
    status text,
    created_at timestamp,
    max_retries int
);
```

### `job_runs` Table
```cql
CREATE TABLE scheduler.job_runs (
    job_id uuid,
    run_id uuid,
    user_id text,
    status text,
    triggered_at timestamp,
    completed_at timestamp,
    output text,
    worker_id text,
    error_message text,
    PRIMARY KEY (job_id, run_id)
);
```

### `job_queue` Table (Time-series)
```cql
CREATE TABLE scheduler.job_queue (
    shard_id int,
    next_fire_at timestamp,
    job_id uuid,
    payload text,
    PRIMARY KEY (shard_id, next_fire_at, job_id)
) WITH CLUSTERING ORDER BY (next_fire_at ASC);
```

---

## 🔧 Configuration

### Environment Variables

**Ingestion Service:**
- `PORT` - HTTP server port (default: 8080)
- `SCYLLA_HOSTS` - Scylla contact points
- `KAFKA_BROKERS` - Kafka broker addresses
- `S3_ENDPOINT` - S3 endpoint URL

**Worker Service:**
- `SCYLLA_HOSTS` - Scylla contact points
- `SQS_ENDPOINT` - SQS endpoint URL
- `QUEUE_NAME` - SQS queue name
- `S3_ENDPOINT` - S3 endpoint URL

### Docker Compose Configuration
All services are configured via `docker-compose.yml`. Customize environment variables, resource limits, and port mappings as needed.

---

## 🛠️ Development

### Project Structure
```
├── cmd/               # Service entry points
│   ├── ingestion/     # HTTP API service
│   ├── writer/        # Kafka consumer service
│   ├── coordinator/   # Leader election service
│   ├── picker/        # Job queue scanner
│   └── worker/        # Job executor
├── pkg/               # Shared packages
│   ├── infra/         # Infrastructure clients (Scylla, Kafka, SQS, S3)
│   └── observability/ # Metrics & monitoring
├── tests/             # Test suites
│   ├── integration/   # End-to-end tests
│   └── chaos/         # Resilience tests
├── docs/              # Documentation
├── db/                # Database schemas
└── docker-compose.yml # Service orchestration
```

### Build Services
```bash
# Build all services
docker-compose build

# Build specific service
docker-compose build worker-service
```

---

## 🐛 Troubleshooting

### Jobs Not Executing
1. **Check Writer Logs:** `docker logs scheduler-writer`
2. **Verify job_queue:** 
   ```bash
   docker exec -it scheduler-scylla cqlsh -e \
     "SELECT COUNT(*) FROM scheduler.job_queue;"
   ```
3. **Check Picker Logs:** `docker logs scheduler-picker`
4. **Verify SQS Messages:** `curl http://localhost:9325/queues/scheduler-job-queue`

### Services Not Starting
```bash
# Check specific service logs
docker logs scheduler-<service-name>

# Restart service
docker-compose restart <service-name>

# Fresh start
docker-compose down -v && docker-compose up -d
```

### Performance Issues
- **Scale Workers:** Increase worker replicas in docker-compose.yml
- **Check Metrics:** Review Prometheus/Grafana for bottlenecks
- **Database Tuning:** Adjust Scylla memory/CPU in docker-compose.yml

---

## 📚 Additional Resources

- **[Setup Guide](./docs/guides/setup_and_test.md)** - Detailed setup instructions
- **[Test Results](./TEST_RESULTS.md)** - Complete test verification
- **[Demo Guide](./DEMO.md)** - End-to-end demonstration
- **[Notion Documentation](https://www.notion.so/Job-Scheduler-2bf98102139b80bb85fde3c663647f80?source=copy_link)** - Architecture & design

---

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🏆 Performance Highlights

- **Throughput:** 16.30 jobs/sec (load test with 100 concurrent jobs)
- **Reliability:** 11/11 tests passing (9 integration + 2 chaos)
- **Command Execution:** ~1.5s average latency
- **Shell Scripts:** ~2.5s average execution time
- **Fault Tolerance:** Automatic recovery from picker/worker failures
- **S3 Offloading:** Seamless handling of payloads >1KB

---

## 🙏 Acknowledgments

Built with:
- [Go](https://golang.org/) - Programming language
- [ScyllaDB](https://www.scylladb.com/) - High-performance NoSQL database
- [Apache Kafka](https://kafka.apache.org/) - Event streaming platform
- [Etcd](https://etcd.io/) - Distributed coordination
- [Prometheus](https://prometheus.io/) & [Grafana](https://grafana.com/) - Observability stack
- [ElasticMQ](https://github.com/softwaremill/elasticmq) - SQS-compatible message queue
- [LocalStack](https://localstack.cloud/) - AWS service emulation

---

**Ready to schedule jobs at scale? Start now with `docker-compose up -d`** 🚀
