package integration

import (
    "testing"
    "time"
)

func TestScheduledJobExecution(t *testing.T) {
    // 1. Submit Job for 5 seconds later
    delay := 5 * time.Second
    fireAt := time.Now().Add(delay)
    payload := "test-scheduled-" + fireAt.Format(time.RFC3339)
    
    // Format required by Ingestion Service: RFC3339
    jobID := submitJob(t, "integration-test", payload, "", fireAt.Format(time.RFC3339))

    t.Logf("Submitted scheduled job: %s (Fire At: %s)", jobID, fireAt)

    // 2. Verify it does NOT run immediately
    // Wait 2 seconds (still before fireAt)
    time.Sleep(2 * time.Second)
    
    // Check job_runs (should be empty/not found for this job run)
    // Note: Our waitForJobCompletion waits for success. Here we want to assert explicitly.
    // We can just query `job_runs`.
    iter := scyllaClient.Session.Query(`SELECT status FROM job_runs WHERE job_id = ? ALLOW FILTERING`, jobID).Iter()
    if iter.NumRows() > 0 {
        t.Errorf("Job %s ran too early!", jobID)
    }
    iter.Close()

    // 3. Wait past the fire time (total 7s > 5s delay)
    // Need to account for Picker polling interval (1s) + processing
    waitForJobCompletion(t, jobID, 10*time.Second)

    t.Logf("Scheduled job %s completed successfully after delay", jobID)
}
