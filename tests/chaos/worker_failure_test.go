package chaos

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os/exec"
    "strings"
    "testing"
    "time"
)

func TestWorkerFailure(t *testing.T) {
    // 1. Submit long running job (sleep:30s)
    payload := "sleep:30s"
    jobID := submitJob(t, "chaos-test", payload, "", "")
    t.Logf("Submitted job %s with 30s sleep", jobID)

    // 2. Wait for it to be picked up (~2s)
    time.Sleep(3 * time.Second)

    // Verify running
    if !isContainerRunning(t, "scheduler-worker") {
        t.Fatal("Worker should be running")
    }

    // 3. Kill Worker
    t.Log("Killing scheduler-worker container...")
    if err := exec.Command("docker", "kill", "scheduler-worker").Run(); err != nil {
        t.Fatalf("Failed to kill worker: %v", err)
    }

    // 4. Wait a bit (ensure it's dead)
    time.Sleep(5 * time.Second)

    // Verify dead
    if isContainerRunning(t, "scheduler-worker") {
        t.Fatal("Worker should be dead but is still running!")
    }

    // 5. Restart Worker
    t.Log("Restarting scheduler-worker container...")
    if err := exec.Command("docker", "start", "scheduler-worker").Run(); err != nil {
        t.Fatalf("Failed to start worker: %v", err)
    }

    // 6. Verify Completion
    // Total time: 33s sleep + 5s downtime + ~40s rebalance + startup.
    t.Log("Waiting for job completion/recovery...")
    waitForJobCompletion(t, jobID, 90*time.Second)
    
    t.Log("Job recovered and completed successfully!")
}

func isContainerRunning(t *testing.T, containerName string) bool {
    out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName).Output()
    if err != nil {
        t.Fatalf("Failed to inspect container %s: %v", containerName, err)
    }
    return strings.TrimSpace(string(out)) == "true"
}

// Helpers (Duplicated from integration suite for independence, or could share if refactored)
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

func waitForJobCompletion(t *testing.T, jobID string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    ticker := time.NewTicker(1 * time.Second)
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
