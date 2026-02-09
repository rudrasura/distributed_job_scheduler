package integration

import (
    "os/exec"
    "testing"
    "time"
)

func TestSQSRestart(t *testing.T) {
    // 1. Submit a job to ensure system is initially healthy
    jobID1 := submitJob(t, "chaos-sqs", "sleep:1s", "", time.Now().Add(2*time.Second).Format(time.RFC3339))
    waitForJobCompletion(t, jobID1, 20*time.Second)

    // 2. Restart SQS container
    t.Log("Restarting SQS container...")
    cmd := exec.Command("docker", "restart", "scheduler-sqs")
    if err := cmd.Run(); err != nil {
        t.Fatalf("Failed to restart SQS: %v", err)
    }
    
    // Give it a moment to come back up
    time.Sleep(5 * time.Second)

    // 3. Submit another job
    // This verifies that Picker and Worker can reconnect to SQS
    jobID2 := submitJob(t, "chaos-sqs", "sleep:1s", "", time.Now().Add(2*time.Second).Format(time.RFC3339))
    
    // 4. Verify completion
    waitForJobCompletion(t, jobID2, 30*time.Second)
}
