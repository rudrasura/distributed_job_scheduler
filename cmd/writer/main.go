package main

import (
    "encoding/json"
    "log"
    "os"
    "strings"
    "time"

    "distributed_job_scheduler/pkg/infra"
    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type JobSubmissionEvent struct {
    JobID      string `json:"job_id"`
    ProjectID  string `json:"project_id"`
    NextFireAt string `json:"next_fire_at"`
    ShardID    int    `json:"shard_id"`
    Payload    string `json:"payload"`
}

var (
    scyllaClient *infra.ScyllaClient
    kafkaConsumer *infra.KafkaConsumer
)

func main() {
    log.Println("Starting Writer Service...")

    initInfra()
    defer closeInfra()

    log.Println("Writer Service started. Consuming messages...")
    
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

    // Kafka
    kafkaBrokers := os.Getenv("KAFKA_BROKERS")
    if kafkaBrokers == "" { kafkaBrokers = "scheduler-kafka:29092" }
    kafkaConsumer, err = infra.NewKafkaConsumer(kafkaBrokers, "writer-group", "job-submissions")
    if err != nil {
        log.Fatalf("Failed to connect to Kafka: %v", err)
    }
    log.Println("Connected to Kafka")
}

func closeInfra() {
    if scyllaClient != nil { scyllaClient.Close() }
    if kafkaConsumer != nil { kafkaConsumer.Close() }
}

func runLoop() {
    for {
        msg, err := kafkaConsumer.ReadMessage(100 * time.Millisecond)
        if err != nil {
            // Ignore timeouts, log others
            if err.(kafka.Error).Code() != kafka.ErrTimedOut {
                log.Printf("Consumer error: %v (%v)", err, msg)
            }
            continue
        }

        processMessage(msg)
    }
}

func processMessage(msg *kafka.Message) {
    var event JobSubmissionEvent
    if err := json.Unmarshal(msg.Value, &event); err != nil {
        log.Printf("Failed to unmarshal message: %v", err)
        return
    }

    nextFireAt, err := time.Parse(time.RFC3339, event.NextFireAt)
    if err != nil {
        log.Printf("Failed to parse next_fire_at: %v", err)
        return
    }

    // Insert into job_queue
    // PK: ((shard_id), next_fire_at, job_id)
    query := `INSERT INTO job_queue (shard_id, next_fire_at, job_id, status) VALUES (?, ?, ?, ?)`
    err = scyllaClient.Session.Query(query, 
        event.ShardID, 
        nextFireAt, 
        event.JobID, 
        "PENDING").Exec()

    if err != nil {
        log.Printf("Scylla write failed for job %s: %v", event.JobID, err)
        // Ensure retry logic or dead-letter queue in production
        return
    }

    log.Printf("Processed job %s (Shard: %d, FireAt: %s)", event.JobID, event.ShardID, event.NextFireAt)
}
