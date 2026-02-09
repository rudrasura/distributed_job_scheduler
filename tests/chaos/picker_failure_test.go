package chaos

import (
    "os/exec"
    "testing"
    "time"
)

func TestPickerFailure(t *testing.T) {
    // 1. Submit scheduled job for 10s later
    fireAt := time.Now().Add(10 * time.Second)
    payload := "test-picker-recovery"
    
    // We reuse submitJob from worker_failure_test.go (it's in the same package 'chaos')
    jobID := submitJob(t, "chaos-test", payload, "", fireAt.Format(time.RFC3339))
    t.Logf("Submitted job %s scheduled for %s", jobID, fireAt)

    // 2. Kill Picker
    t.Log("Killing scheduler-picker container...")
    if err := exec.Command("docker", "kill", "scheduler-picker").Run(); err != nil {
        t.Fatalf("Failed to kill picker: %v", err)
    }

    // 3. Wait past the fire time (total 15s wait)
    t.Log("Waiting past schedule time (with Picker dead)...")
    time.Sleep(15 * time.Second)
    
    // 4. Verify NOT executed
    // Check job_runs
    if hasRun(t, jobID) {
        t.Fatal("Job executed while Picker was dead!")
    }

    // 5. Restart Picker
    t.Log("Restarting scheduler-picker container...")
    if err := exec.Command("docker", "start", "scheduler-picker").Run(); err != nil {
        t.Fatalf("Failed to start picker: %v", err)
    }

    // 6. Verify Completion
    // Picker needs to startup, scan, and publish.
    t.Log("Waiting for job completion/recovery...")
    waitForJobCompletion(t, jobID, 30*time.Second) 
    
    t.Log("Job recovered and completed successfully!")
}

func hasRun(t *testing.T, jobID string) bool {
    iter := scyllaClient.Session.Query(`SELECT status FROM job_runs WHERE job_id = ? ALLOW FILTERING`, jobID).Iter()
    defer iter.Close()
    return iter.NumRows() > 0
}
