package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"distributed_job_scheduler/pkg/infra"
	"distributed_job_scheduler/pkg/observability"
)

// JobRequest represents the client submission
type JobRequest struct {
    ProjectID    string `json:"project_id"`
    Payload      string `json:"payload"`
    CronSchedule string `json:"cron_schedule"`
    NextFireAt   string `json:"next_fire_at"` // ISO8601
    MaxRetries   int    `json:"max_retries"`
}

// JobResponse represents the success response
type JobResponse struct {
    JobID   string `json:"job_id"`
    Status  string `json:"status"`
    Message string `json:"message"`
}

var (
    scyllaClient *infra.ScyllaClient
    redisClient  *infra.RedisClient
    kafkaProducer *infra.KafkaProducer
    s3Client     *infra.S3Client
)

func main() {
    log.Println("Starting Ingestion Service...")

    // 1. Initialize Infrastructure
    initInfra()
    defer closeInfra()

    // 2. Setup Router
    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/submit", submitHandler)
    http.HandleFunc("/job", getJobHandler)
    http.HandleFunc("/jobs", getJobsHandler)

    // 3. Metrics Endpoint (separate port)
    go func() {
        metricshttp := http.NewServeMux()
        metricshttp.Handle("/metrics", promhttp.Handler())
        log.Println("Metrics endpoint listening on :8081")
        if err := http.ListenAndServe(":8081", metricshttp); err != nil {
            log.Fatalf("Metrics server failed: %v", err)
        }
    }()

    // 4. Start Server
    log.Println("Ingestion Service listening on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

// ... (initInfra, etc.)

func getJobHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    jobID := r.URL.Query().Get("id")
    if jobID == "" {
        http.Error(w, "Missing id parameter", http.StatusBadRequest)
        return
    }

    var job JobRequest
    var status string
    var nextFireAt time.Time
    var createdAt time.Time

    query := `SELECT project_id, payload, cron_schedule, next_fire_at, status, created_at FROM jobs WHERE job_id = ?`
    err := scyllaClient.Session.Query(query, jobID).Scan(
        &job.ProjectID, &job.Payload, &job.CronSchedule, &nextFireAt, &status, &createdAt)

    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            http.Error(w, "Job not found", http.StatusNotFound)
            return
        }
        log.Printf("Scylla query failed: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    
    // Construct response structure (could map to a specific DTO)
    resp := map[string]interface{}{
        "job_id": jobID,
        "project_id": job.ProjectID,
        "payload": job.Payload,
        "status": status,
        "next_fire_at": nextFireAt,
        "created_at": createdAt,
    }
    if job.CronSchedule != "" {
        resp["cron_schedule"] = job.CronSchedule
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func getJobsHandler(w http.ResponseWriter, r *http.Request) {
	status := "200"
	start := time.Now()
	defer func() {
		observability.HttpRequestDuration.WithLabelValues(r.Method, "/jobs").Observe(time.Since(start).Seconds())
		observability.HttpRequestsTotal.WithLabelValues(r.Method, "/jobs", status).Inc()
	}()

	if r.Method != http.MethodGet {
		status = "405"
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		status = "400"
		http.Error(w, "Missing X-User-ID header", http.StatusBadRequest)
		return
	}

	// Query user_jobs table for efficient lookups by User ID
	query := `SELECT job_id, status, next_fire_at, created_at FROM user_jobs WHERE user_id = ?`
	iter := scyllaClient.Session.Query(query, userID).Iter()
	
	var jobs []map[string]string
	var id gocql.UUID
	var statusDB string
	var nextFireAt, createdAt *time.Time

	for iter.Scan(&id, &statusDB, &nextFireAt, &createdAt) {
		job := map[string]string{
			"job_id": id.String(),
			"status": statusDB,
		}
		
		// Handle NULL next_fire_at (for immediate jobs)
		if nextFireAt != nil {
			job["next_fire_at"] = nextFireAt.Format(time.RFC3339)
		} else {
			job["next_fire_at"] = ""
		}
		
		// Handle NULL created_at (shouldn't happen, but be safe)
		if createdAt != nil {
			job["created_at"] = createdAt.Format(time.RFC3339)
		} else {
			job["created_at"] = ""
		}
		
		jobs = append(jobs, job)
	}

	if err := iter.Close(); err != nil {
		log.Printf("Scylla iteration failed: %v", err)
		status = "500"
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func initInfra() {
	var err error

	// Scylla
	scyllaHosts := strings.Split(os.Getenv("SCYLLA_HOSTS"), ",")
	if len(scyllaHosts) == 0 || scyllaHosts[0] == "" {
		scyllaHosts = []string{"scheduler-scylla"}
	}
	scyllaClient, err = infra.NewScyllaClient(scyllaHosts, "scheduler")
	if err != nil {
		log.Fatalf("Failed to connect to Scylla: %v", err)
	}
	log.Println("Connected to ScyllaDB")

	// Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "scheduler-redis:6379"
	}
	redisClient, err = infra.NewRedisClient(redisAddr, "", 0)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Kafka
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "scheduler-kafka:29092"
	}
	kafkaProducer, err = infra.NewKafkaProducer(kafkaBrokers, "job-submissions")
	if err != nil {
		log.Fatalf("Failed to connect to Kafka: %v", err)
	}
	log.Println("Connected to Kafka")

	// S3
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		s3Endpoint = "http://scheduler-s3:4566"
	}
	s3Client, err = infra.NewS3Client(s3Endpoint)
	if err != nil {
		log.Fatalf("Failed to connect to S3: %v", err)
	}
	// Ensure bucket exists
	if err := s3Client.EnsureBucket("job-payloads"); err != nil {
		log.Printf("Failed to ensure S3 bucket: %v", err)
	}
	log.Println("Connected to S3")
}

func closeInfra() {
	if scyllaClient != nil {
		scyllaClient.Close()
	}
	if redisClient != nil {
		redisClient.Close()
	}
	if kafkaProducer != nil {
		kafkaProducer.Close()
	}
	// S3 client (AWS SDK v2) doesn't strictly need Close
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200" // Default success status
	defer func() {
		observability.HttpRequestDuration.WithLabelValues(r.Method, "/submit").Observe(time.Since(start).Seconds())
		observability.HttpRequestsTotal.WithLabelValues(r.Method, "/submit", status).Inc()
	}()

	if r.Method != http.MethodPost {
		status = "405"
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status = "400"
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()

	// Consistent timestamp for both tables
	now := time.Now()
	shardID := int(now.UnixNano()) % 1024 // Simple sharding for now
	userID := r.Header.Get("X-User-ID")
	observability.JobsCreatedTotal.WithLabelValues(userID).Inc()

	// S3 Offloading Logic
	payload := req.Payload
	if len(payload) > 1024 { // Example threshold: offload payloads larger than 1KB
		s3Start := time.Now()
		key := fmt.Sprintf("payloads/%s", jobID)

		_, err := s3Client.Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String("job-payloads"),
			Key:    aws.String(key),
			Body:   strings.NewReader(payload),
		})

		observability.PayloadStorageDuration.WithLabelValues("s3").Observe(time.Since(s3Start).Seconds())
		observability.S3OperationsTotal.WithLabelValues("upload").Inc()

		if err != nil {
			log.Printf("Failed to upload payload to S3: %v", err)
			status = "500"
			http.Error(w, "Failed to store payload", http.StatusInternalServerError)
			return
		}
		payload = "s3:" + key // Store S3 reference in Scylla
	} else {
		observability.PayloadStorageDuration.WithLabelValues("scylla").Observe(0) // Record a tiny duration for direct storage
	}

	// Parse NextFireAt
	var nextFireAt time.Time
	var err error
	if req.NextFireAt != "" {
		nextFireAt, err = time.Parse(time.RFC3339, req.NextFireAt)
		if err != nil {
			status = "400"
			http.Error(w, "Invalid next_fire_at format (RFC3339 required)", http.StatusBadRequest)
			return
		}
	} else {
		nextFireAt = time.Now()
	}

	// 1. Persist to Scylla (Main Table)
	query := `INSERT INTO jobs (job_id, project_id, user_id, payload, cron_schedule, next_fire_at, status, created_at, updated_at, max_retries, retry_count, shard_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	err = scyllaClient.Session.Query(query,
		jobID,
		req.ProjectID,
		userID,
		payload,
		req.CronSchedule,
		nextFireAt,
		"PENDING",
		now, // created_at
		now, // updated_at
		req.MaxRetries,
		0, // retry_count
		shardID).Exec()

	if err != nil {
		log.Printf("Scylla write to jobs failed: %v", err)
		status = "500"
		http.Error(w, "Internal Storage Error", http.StatusInternalServerError)
		return
	}

	// 1.5 Persist to User Lookup Table (Manual Index)
	if userID != "" {
		userQuery := `INSERT INTO user_jobs (user_id, created_at, job_id, status, next_fire_at) VALUES (?, ?, ?, ?, ?)`
		err = scyllaClient.Session.Query(userQuery, userID, now, jobID, "PENDING", nextFireAt).Exec()
		if err != nil {
			log.Printf("Failed to write to user_jobs (non-fatal): %v", err)
			// Proceeding because main write succeeded
		}
	}

	// 2. Publish to Kafka
	event := map[string]interface{}{
		"job_id":       jobID,
		"project_id":   req.ProjectID,
		"user_id":      userID,
		"next_fire_at": nextFireAt.Format(time.RFC3339),
		"submitted_at": now.Format(time.RFC3339),
		"shard_id":     shardID,
		"payload":      payload, // This will be the S3 reference if offloaded
		"max_retries":  req.MaxRetries,
	}
	eventBytes, _ := json.Marshal(event)

	kafkaStart := time.Now()
	err = kafkaProducer.Publish(jobID, eventBytes)
	observability.KafkaPublishDuration.Observe(time.Since(kafkaStart).Seconds())

	if err != nil {
		log.Printf("Kafka publish failed: %v", err)
		observability.KafkaPublishErrors.Inc()
		// Note: In a real system, we might want to rollback Scylla or mark as error,
		// but for now we proceed.
		status = "500"
		http.Error(w, "Internal Messaging Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(JobResponse{
		JobID:   jobID,
		Status:  "Submitted",
		Message: "Job submitted successfully",
	})
}
