package integration

import (
    "testing"
    "time"
)

func TestImmediateJobExecution(t *testing.T) {
    // 1. Submit Job
    payload := "test-immediate-" + time.Now().Format(time.RFC3339)
    jobID := submitJob(t, "integration-test", payload, "", "")

    t.Logf("Submitted immediate job: %s", jobID)

    // 2. Verify PENDING status in `jobs` table
    initialStatus := GetJobStatus(t, jobID)
    if initialStatus != "PENDING" {
        t.Errorf("Expected initial status PENDING, got %s", initialStatus)
    }

    // 3. Wait for COMPLETION in `job_runs` table
    // Writer -> Queue -> Picker -> Kafka -> Worker -> DB
    waitForJobCompletion(t, jobID, 15*time.Second)
    
    t.Logf("Job %s completed successfully", jobID)
}
