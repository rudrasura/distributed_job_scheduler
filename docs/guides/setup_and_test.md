# Distributed Job Scheduler - Setup & User Guide

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.22+
- `cqlsh` (optional, for DB debugging)

### Start Services
```bash
docker-compose up -d
```
All services (Ingestion, Writer, Picker, Worker) and infrastructure (Scylla, Kafka, Etcd, Redis, Prometheus) will start.

### Stop Services
```bash
docker-compose down
```

---

## 🧪 Running Tests

### 1. Integration Tests (End-to-End)
Verifies the full job lifecycle (Immediate, Scheduled, Recurring).
```bash
go test -v ./tests/integration/...
```

### 2. Load Tests (Stability)
Submits 100 concurrent jobs to verify system stability.
```bash
go test -v -run TestLoad ./tests/integration/...
```

### 3. Chaos Tests (Resilience)
Verifies recovery from component failures. **Warning**: Kills Docker containers.
```bash
go test -v ./tests/chaos/...
```

### 4. Test Coverage
Run the coverage script to generate a report:
```bash
make coverage
```
*(See `Makefile` or `scripts/coverage.sh`)*

---

## 📡 API Reference (Ingestion Service)
Base URL: `http://localhost:8080`

### 1. Submit Job
**POST** `/submit`
- Headers: `X-User-ID: user-123` (Recommended), `X-Project-ID: project-123`
```json
{
  "project_id": "project-123",
  "payload": "do-work",
  "cron_schedule": "@every 5m", 
  "next_fire_at": "2024-01-01T12:00:00Z"
}
```

### 2. Get Job Details
**GET** `/job?id=<job_id>`

### 3. List Jobs by User
**GET** `/jobs`
- Headers: `X-User-ID: user-123` (Required)

---

## 📊 Observability

### Prometheus
- URL: `http://localhost:9090`
- Metric: `job_scheduler_jobs_processed_total`

### Grafana
- URL: `http://localhost:3000`
- User/Pass: `admin` / `admin` (default)
- **Dashboards**:
  1. Login to Grafana.
  2. Go to **Dashboards**.
  3. Import the `Job Scheduler Dashboard` (if configured) or create new panels querying Prometheus metrics.
