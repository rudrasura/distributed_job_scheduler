package integration

import (
    "testing"
    "time"
)

func TestRecurringJobExecution(t *testing.T) {
    // 1. Submit Recurring Job (@every 2s)
    // We use @every 2s to ensure it runs quickly for the test
    schedule := "@every 2s"
    payload := "test-recurring-" + time.Now().Format(time.RFC3339)
    
    // Immediate first run
    jobID := submitJob(t, "integration-test", payload, schedule, "")

    t.Logf("Submitted recurring job: %s (Schedule: %s)", jobID, schedule)

    // 2. Wait for Run 1
    t.Log("Waiting for Run 1...")
    waitForJobCompletion(t, jobID, 10*time.Second) 
    
    // 3. Verify Job Status and NextFireAt updated
    // Job status might be PENDING again by the time we check, or COMPLETED if we check run history.
    // The 'jobs' table status should be PENDING (waiting for next run).
    // The 'next_fire_at' should be in the future.
    
    // Allow small delay for Worker to update 'jobs' table after "Completion"
    time.Sleep(1 * time.Second)

    jobDetails := getJobDetails(t, jobID)
    if jobDetails.Status != "PENDING" {
         t.Errorf("Expected job status PENDING (rescheduled), got %s", jobDetails.Status)
    }
    
    if jobDetails.NextFireAt.Before(time.Now()) {
        t.Errorf("Expected next_fire_at to be in future, got %s", jobDetails.NextFireAt)
    }
    
    t.Logf("Run 1 complete. Next run at: %s", jobDetails.NextFireAt)

    // 4. Wait for Run 2
    // We need to wait until next_fire_at, then Picker picks it, then Worker runs it.
    // Since we used @every 2s, it should be very soon.
    
    // We can check `job_runs` count.
    t.Log("Waiting for Run 2...")
    
    // Wait for at least 3 seconds to ensure 2nd run triggers
    time.Sleep(3 * time.Second)
    
    // Check if we have at least 2 runs
    iter := scyllaClient.Session.Query(`SELECT run_id FROM job_runs WHERE job_id = ? ALLOW FILTERING`, jobID).Iter()
    runCount := iter.NumRows()
    iter.Close()
    
    if runCount < 2 {
        t.Errorf("Expected at least 2 runs, got %d", runCount)
    } else {
        t.Logf("Verified %d runs for recurring job", runCount)
    }
}
