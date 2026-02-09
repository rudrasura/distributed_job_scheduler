package integration

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"

    "distributed_job_scheduler/pkg/infra"
    "github.com/gocql/gocql"
)

// Shared Clients
var (
    scyllaClient *infra.ScyllaClient
)

func TestMain(m *testing.M) {
    // Setup
    var err error
    scyllaHosts := []string{"localhost"} // Assume port 9042 exposed
    scyllaClient, err = infra.NewScyllaClient(scyllaHosts, "scheduler")
    if err != nil {
        fmt.Printf("Failed to connect to Scylla (localhost:9042): %v\n", err)
        os.Exit(1)
    }

    // Run Tests
    code := m.Run()

    // Teardown
    scyllaClient.Close()
    os.Exit(code)
}

// Helpers

func waitForJobCompletion(t *testing.T, jobID string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    query := `SELECT status FROM job_runs WHERE job_id = ? ALLOW FILTERING`

    for time.Now().Before(deadline) {
        iter := scyllaClient.Session.Query(query, jobID).Iter()
        var status string
        if iter.Scan(&status) {
            iter.Close()
            if status == "COMPLETED" {
                return // Success
            }
        } else {
            iter.Close()
        }
        
        <-ticker.C
    }
    t.Fatalf("Job %s did not complete within %v", jobID, timeout)
}

func submitJob(t *testing.T, projectID, payload, schedule, nextFireAt string) string {
    url := "http://localhost:8080/submit"
    body := fmt.Sprintf(`{"project_id": "%s", "payload": "%s", "cron_schedule": "%s", "next_fire_at": "%s"}`, 
        projectID, payload, schedule, nextFireAt)

    resp, err := http.Post(url, "application/json", strings.NewReader(body))
    if err != nil {
        t.Fatalf("Failed to submit job: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        bodyBytes, _ := io.ReadAll(resp.Body)
        t.Fatalf("Submit failed (Status %d): %s", resp.StatusCode, string(bodyBytes))
    }

    var result map[string]string
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        t.Fatalf("Failed to decode response: %v", err)
    }

    return result["job_id"]
}

func GetJobStatus(t *testing.T, jobID string) string {
    var status string
    // Check main jobs table
    err := scyllaClient.Session.Query(`SELECT status FROM jobs WHERE job_id = ?`, jobID).Scan(&status)
    if err != nil {
        if err == gocql.ErrNotFound {
            return "NOT_FOUND"
        }
        t.Fatalf("Failed to get job status from DB: %v", err)
    }
    return status
}
