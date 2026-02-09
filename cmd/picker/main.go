package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "strings"
    "strconv"
    "time"

    "distributed_job_scheduler/pkg/infra"
    "distributed_job_scheduler/pkg/observability"
    "github.com/google/uuid"
    "github.com/gocql/gocql"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
    scyllaClient  *infra.ScyllaClient
    sqsClient     *infra.SQSClient
    etcdClient    *infra.EtcdClient
)

func main() {
	log.Println("Starting Picker Service...")

	initInfra()
	defer closeInfra()

	// Start metrics endpoint
	go func() {
		metricshttp := http.NewServeMux()
		metricshttp.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics endpoint listening on :8082")
		if err := http.ListenAndServe(":8082", metricshttp); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()

	runPickerLoop()
}

func initInfra() {
    var err error

    // Scylla
    scyllaHosts := strings.Split(os.Getenv("SCYLLA_HOSTS"), ",")
    if len(scyllaHosts) == 0 { 
        scyllaHosts = []string{"scheduler-scylla"} 
    }
    scyllaClient, err = infra.NewScyllaClient(scyllaHosts, "scheduler")
    if err != nil {
        log.Fatalf("Failed to connect to Scylla: %v", err)
    }
    log.Println("Connected to ScyllaDB")

    // SQS
    sqsEndpoint := os.Getenv("SQS_ENDPOINT")
    if sqsEndpoint == "" { sqsEndpoint = "http://scheduler-sqs:9324" }
    sqsClient, err = infra.NewSQSClient(sqsEndpoint, "job-queue")
    if err != nil {
        log.Fatalf("Failed to connect to SQS: %v", err)
    }
    log.Println("Connected to SQS")

    // Etcd
    endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
    if len(endpoints) == 0 { 
        endpoints = []string{"scheduler-etcd:2379"} 
    }
    etcdClient, err = infra.NewEtcdClient(endpoints)
    if err != nil {
        log.Fatalf("Failed to connect to Etcd: %v", err)
    }
    log.Println("Connected to Etcd")
}

func closeInfra() {
    if scyllaClient != nil { scyllaClient.Close() }
    if etcdClient != nil { etcdClient.Close() }
}

func runPickerLoop() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        <-ticker.C
        // For Phase 4.3, assume we own ALL shards (0-1023)
        // In real impl, we fetch assignments from Etcd
        for shardID := 0; shardID < 1024; shardID++ {
             start := time.Now()
             scanShard(shardID)
             observability.ScanCycleDuration.WithLabelValues(strconv.Itoa(shardID)).Observe(time.Since(start).Seconds())
             observability.PickerScansTotal.WithLabelValues(strconv.Itoa(shardID)).Inc()
        }
    }
}

func scanShard(shardID int) {
    now := time.Now()
    // 1. Get candidate jobs from job_queue (metadata only)
    query := `SELECT job_id, next_fire_at FROM job_queue WHERE shard_id = ? AND next_fire_at <= ? ALLOW FILTERING`
    
    iter := scyllaClient.Session.Query(query, shardID, now).Iter()
    
    var jobID gocql.UUID
    var nextFireAt time.Time
    
    // Store candidates to process
    type Candidate struct {
        ID     gocql.UUID
        FireAt time.Time
    }
    var candidates []Candidate

    for iter.Scan(&jobID, &nextFireAt) {
        candidates = append(candidates, Candidate{ID: jobID, FireAt: nextFireAt})
    }
    
    if err := iter.Close(); err != nil {
        log.Printf("Iterator error on shard %d: %v", shardID, err)
    }

    observability.JobsScannedTotal.Add(float64(len(candidates)))

    // 2. Process candidates
    for _, cand := range candidates {
        var payload, projectID, cronSchedule string
        var maxRetries int
        var userID string
        
        // Fetch full details from 'jobs' table
        // Updated to include user_id and max_retries
        err := scyllaClient.Session.Query(`SELECT payload, project_id, cron_schedule, user_id, max_retries FROM jobs WHERE job_id = ?`, cand.ID).Scan(&payload, &projectID, &cronSchedule, &userID, &maxRetries)
        if err != nil {
            log.Printf("Failed to fetch details for job %s: %v", cand.ID, err)
            continue
        }

        log.Printf("Picking job %s (Shard: %d)", cand.ID, shardID)
        
        // 3. Publish to SQS
        event := map[string]interface{}{
            "job_id": cand.ID.String(),
            "run_id": uuid.New().String(),
            "status": "STARTED",
            "executed_at": time.Now().Format(time.RFC3339),
            "payload": payload,
            "project_id": projectID,
            "cron_schedule": cronSchedule,
            "user_id": userID,
            "max_retries": maxRetries,
        }
        eventBytes, _ := json.Marshal(event)
        
        log.Printf("[DEBUG] Publishing job %s to SQS (QueueURL: %s, Payload size: %d bytes)", cand.ID, sqsClient.QueueURL, len(eventBytes))
        sqsStart := time.Now()
        err = sqsClient.SendMessage(context.TODO(), string(eventBytes))
        observability.SQSEnqueueDuration.Observe(time.Since(sqsStart).Seconds())

        if err != nil {
            log.Printf("Failed to publish job execution to SQS: %v", err)
            observability.SQSEnqueueErrors.Inc()
            continue // Don't delete from queue if SQS publish failed
        }
        log.Printf("[DEBUG] Successfully published job %s to SQS", cand.ID)
        observability.JobsEnqueuedTotal.Inc()

        // 4. Delete from job_queue ONLY if SQS publish succeeded
        delQuery := `DELETE FROM job_queue WHERE shard_id = ? AND next_fire_at = ? AND job_id = ?`
        if err := scyllaClient.Session.Query(delQuery, shardID, cand.FireAt, cand.ID).Exec(); err != nil {
            log.Printf("Failed to delete job from queue: %v", err)
        }
    }
}
