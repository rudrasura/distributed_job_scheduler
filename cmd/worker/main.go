package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "math/rand"
    "net/http"
    "os"
    "os/exec"
    "strings"
    "time"

    "distributed_job_scheduler/pkg/infra"
    "distributed_job_scheduler/pkg/observability"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    "github.com/robfig/cron/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type JobExecutionEvent struct {
    JobID      string `json:"job_id"`
    RunID      string `json:"run_id"`
    Status     string `json:"status"`
    Payload    string `json:"payload"`
    ProjectID  string `json:"project_id"`
    UserID     string `json:"user_id"`
    ExecutedAt string `json:"executed_at"`
    CronSchedule string `json:"cron_schedule"`
}

var (
    scyllaClient *infra.ScyllaClient
    sqsClient    *infra.SQSClient
    s3Client     *infra.S3Client
    parser        = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
)

func main() {
    log.Println("Starting Worker Service...")

    // Init Metrics
	go func() {
		metricshttp := http.NewServeMux()
		metricshttp.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics endpoint listening on :8083")
		if err := http.ListenAndServe(":8083", metricshttp); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()

    initInfra()
    defer closeInfra()

    log.Println("Worker Service started. Polling SQS...")
    
    runLoop()
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

    // S3
    s3Endpoint := os.Getenv("S3_ENDPOINT")
    if s3Endpoint == "" { s3Endpoint = "http://scheduler-s3:4566" }
    s3Client, err = infra.NewS3Client(s3Endpoint)
    if err != nil {
        log.Fatalf("Failed to connect to S3: %v", err)
    }
    log.Println("Connected to S3")
}

func closeInfra() {
    if scyllaClient != nil { scyllaClient.Close() }
    // SQS/S3 clients usually don't need close
}

func runLoop() {
    ctx := context.TODO() // In real app, use cancellable context
    for {
        // Long polling
        resp, err := sqsClient.ReceiveMessages(ctx, 10, 5) // max 10 messages, 5s wait
        if err != nil {
            log.Printf("SQS Receive error: %v", err)
            time.Sleep(1 * time.Second)
            continue
        }

        if len(resp.Messages) == 0 {
            continue
        }

        for _, msg := range resp.Messages {
            processMessage(ctx, msg)
        }
    }
}

func processMessage(ctx context.Context, msg types.Message) {
    if msg.Body == nil {
        return
    }

    var event JobExecutionEvent
    if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
        log.Printf("Failed to unmarshal message: %v", err)
        // If poison pill, might want to delete or DLQ. For now, leave it (eventual DLQ by SQS)
        return
    }
    
    startExec := time.Now()
    defer func() {
        observability.JobExecutionDuration.Observe(time.Since(startExec).Seconds())
    }()

    // S3 Payload Retrieval
    if strings.HasPrefix(event.Payload, "s3:") {
        observability.S3OperationsTotal.WithLabelValues("download").Inc()
        key := strings.TrimPrefix(event.Payload, "s3:")
        
        out, err := s3Client.Client.GetObject(ctx, &s3.GetObjectInput{
            Bucket: aws.String("job-payloads"),
            Key:    aws.String(key),
        })
        if err != nil {
             log.Printf("Failed to download payload from S3 (%s): %v", key, err)
             observability.JobsExecutedTotal.WithLabelValues("failed").Inc()
             return 
        }
        
        body, err := io.ReadAll(out.Body)
        out.Body.Close()
        if err != nil {
             log.Printf("Failed to read S3 body: %v", err)
             observability.JobsExecutedTotal.WithLabelValues("failed").Inc()
             return
        }
        event.Payload = string(body)
    }

    log.Printf("Executing Job %s (Run %s): %s", event.JobID, event.RunID, event.Payload)

    var jobOutput string
    var jobStatus string
    var errorMessage string

    // Command execution
    if strings.HasPrefix(event.Payload, "cmd:") {
        cmdStr := strings.TrimPrefix(event.Payload, "cmd:")
        log.Printf("Executing command: %s", cmdStr)
        
        cmd := exec.Command("sh", "-c", cmdStr)
        output, err := cmd.CombinedOutput()
        
        if err != nil {
            log.Printf("Command execution failed: %v", err)
            jobStatus = "FAILED"
            errorMessage = fmt.Sprintf("Command failed: %v", err)
            jobOutput = string(output) // May contain stderr
            observability.JobsExecutedTotal.WithLabelValues("failed").Inc()
        } else {
            jobStatus = "COMPLETED"
            jobOutput = string(output)
            log.Printf("Command executed successfully. Output length: %d bytes", len(output))
            observability.JobsExecutedTotal.WithLabelValues("success").Inc()
        }
    } else if strings.HasPrefix(event.Payload, "sleep:") {
        // Sleep simulation
        durationStr := strings.TrimPrefix(event.Payload, "sleep:")
        duration, err := time.ParseDuration(durationStr)
        if err == nil {
            log.Printf("Sleeping for %v as requested...", duration)
            time.Sleep(duration)
        } else {
            log.Printf("Invalid sleep duration: %v, using default", err)
            time.Sleep(50 * time.Millisecond)
        }
        jobStatus = "COMPLETED"
        jobOutput = "Success: " + event.Payload
        observability.JobsExecutedTotal.WithLabelValues("success").Inc()
    } else {
        // Default simulation
        time.Sleep(50 * time.Millisecond)
        jobStatus = "COMPLETED"
        jobOutput = "Success: " + event.Payload
        observability.JobsExecutedTotal.WithLabelValues("success").Inc()
    }

    // Get Worker ID (Hostname)
    workerID, err := os.Hostname()
    if err != nil {
        workerID = "unknown-worker"
    }

    // Record Run
    query := `INSERT INTO job_runs (job_id, run_id, user_id, status, triggered_at, completed_at, output, worker_id, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
    
    now := time.Now()
    startedAt, _ := time.Parse(time.RFC3339, event.ExecutedAt)
    
    err = scyllaClient.Session.Query(query, 
        event.JobID, 
        event.RunID, 
        event.UserID,
        jobStatus, 
        startedAt, 
        now, 
        jobOutput,
        workerID,
        errorMessage).Exec()

    if err != nil {
        log.Printf("Scylla write failed for run %s: %v", event.RunID, err)
        observability.JobsExecutedTotal.WithLabelValues("failed").Inc()
        // Don't delete message so another worker can retry or DLQ
        return
    }

    log.Printf("Job %s Completed with status: %s", event.JobID, jobStatus)

    // Handle Recurring Jobs
    if event.CronSchedule != "" {
        handleReschedule(event)
    } else {
        // For non-recurring jobs, update status to COMPLETED or FAILED
        updateJobStatus(event.JobID, event.UserID, jobStatus)
    }
    
    // Delete Message
    if err := sqsClient.DeleteMessage(ctx, *msg.ReceiptHandle); err != nil {
        log.Printf("Failed to delete message %s: %v", event.JobID, err)
    }
}

func updateJobStatus(jobID, userID, status string) {
	// Update jobs table
	updateJobQuery := `UPDATE jobs SET status = ? WHERE job_id = ?`
	if err := scyllaClient.Session.Query(updateJobQuery, status, jobID).Exec(); err != nil {
		log.Printf("Failed to update job status for job %s: %v", jobID, err)
		return
	}

	// user_jobs table has composite PRIMARY KEY (user_id, created_at, job_id)
	// We need to get created_at first to update it
	var createdAt time.Time
	getCreatedAtQuery := `SELECT created_at FROM user_jobs WHERE user_id = ? AND job_id = ? ALLOW FILTERING`
	if err := scyllaClient.Session.Query(getCreatedAtQuery, userID, jobID).Scan(&createdAt); err != nil {
		log.Printf("Failed to get created_at for user_job %s: %v", jobID, err)
		return
	}

	// Now update with all primary key columns
	updateUserJobQuery := `UPDATE user_jobs SET status = ? WHERE user_id = ? AND created_at = ? AND job_id = ?`
	if err := scyllaClient.Session.Query(updateUserJobQuery, status, userID, createdAt, jobID).Exec(); err != nil {
		log.Printf("Failed to update user_jobs status for job %s: %v", jobID, err)
		return
	}
	
	log.Printf("Updated job %s status to %s", jobID, status)
}

func handleReschedule(event JobExecutionEvent) {
    schedule, err := parser.Parse(event.CronSchedule)
    if err != nil {
        log.Printf("Failed to parse cron schedule '%s' for job %s: %v", event.CronSchedule, event.JobID, err)
        return
    }

    nextFireAt := schedule.Next(time.Now())
    shardID := rand.Intn(1024) // Simple random sharding for now

    log.Printf("Rescheduling job %s to %v (Shard %d)", event.JobID, nextFireAt, shardID)

    // 1. Update 'jobs' table with new next_fire_at
    updateQuery := `UPDATE jobs SET next_fire_at = ?, shard_id = ?, status = 'PENDING' WHERE job_id = ?`
    if err := scyllaClient.Session.Query(updateQuery, nextFireAt, shardID, event.JobID).Exec(); err != nil {
        log.Printf("Failed to update jobs table for rescheduling: %v", err)
        return // Retry logic would go here
    }

    // 2. Insert into 'job_queue'
    queueQuery := `INSERT INTO job_queue (shard_id, next_fire_at, job_id) VALUES (?, ?, ?)`
    if err := scyllaClient.Session.Query(queueQuery, shardID, nextFireAt, event.JobID).Exec(); err != nil {
        log.Printf("Failed to enqueue rescheduled job: %v", err)
    }
}
