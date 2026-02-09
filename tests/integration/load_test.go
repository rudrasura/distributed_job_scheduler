package integration

import (
    "fmt"
    "sync"
    "testing"
    "time"
)

func TestLoad(t *testing.T) {
    totalJobs := 100
    concurrentSubmitters := 10
    jobsPerSubmitter := totalJobs / concurrentSubmitters

    var wg sync.WaitGroup
    jobIDs := make(chan string, totalJobs)

    startTime := time.Now()

    // 1. Concurrent Submission
    t.Logf("Starting submission of %d jobs with %d workers...", totalJobs, concurrentSubmitters)
    for i := 0; i < concurrentSubmitters; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for j := 0; j < jobsPerSubmitter; j++ {
                payload := fmt.Sprintf("load-test-w%d-%d", workerID, j)
                jobID := submitJob(t, "load-test", payload, "", "")
                jobIDs <- jobID
            }
        }(i)
    }
    wg.Wait()
    close(jobIDs)
    submissionDuration := time.Since(startTime)
    t.Logf("Submission complete in %v (%.2f jobs/sec)", submissionDuration, float64(totalJobs)/submissionDuration.Seconds())

    // 2. Wait for Completion
    t.Log("Waiting for completion...")
    
    // We can't use waitForJobCompletion for 100 jobs sequentially (too slow).
    // Instead, query count of COMPLETED jobs for this project_id?
    // ProjectID lookup isn't supported efficiently in current schema without Allow Filtering or secondary index.
    // So we poll efficiently or check specific IDs.
    
    timeout := 60 * time.Second
    deadline := time.Now().Add(timeout)
    
    completedCount := 0
    // Collect all IDs
    var allJobIDs []string
    for id := range jobIDs {
        allJobIDs = append(allJobIDs, id)
    }

    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for time.Now().Before(deadline) {
        completedCount = 0
        // Sample check (checking all 100 is expensive in a loop, but manageable for this scale)
        // Optimization: Use a worker pool to check status concurrently
        var checkWg sync.WaitGroup
        statusChan := make(chan bool, len(allJobIDs))
        
        chunkSize := 10
        for i := 0; i < len(allJobIDs); i += chunkSize {
            end := i + chunkSize
            if end > len(allJobIDs) { end = len(allJobIDs) }
            
            checkWg.Add(1)
            go func(ids []string) {
                defer checkWg.Done()
                for _, id := range ids {
                    // s := GetJobStatus(t, id)
                    // Note: getJobStatus checks 'jobs' table. 
                    // 'jobs' table status updates happen *after* completion by Worker?
                    // Actually Worker updates 'job_runs' to COMPLETED. 
                    // Does it update 'jobs' table status? 
                    // Looking at Worker code: It inserts into 'job_runs'. It DOES NOT update 'jobs' table status to COMPLETED.
                    // Implementation Gap: Worker should probably update `jobs` table to COMPLETED for non-recurring jobs.
                    // But for now, we must check `job_runs`.
                    
                    // Checking `job_runs` for existance
                    if hasCompletedRun(t, id) {
                        statusChan <- true
                    } else {
                        statusChan <- false
                    }
                }
            }(allJobIDs[i:end])
        }
        checkWg.Wait()
        close(statusChan)
        
        for success := range statusChan {
            if success { completedCount++ }
        }

        t.Logf("Completed: %d/%d", completedCount, totalJobs)
        if completedCount == totalJobs {
            break
        }
        <-ticker.C
    }

    totalDuration := time.Since(startTime)
    t.Logf("Test finished in %v", totalDuration)
    t.Logf("End-to-End Throughput: %.2f jobs/sec", float64(totalJobs)/totalDuration.Seconds())

    if completedCount != totalJobs {
        t.Errorf("Timeout expected %d completed, got %d", totalJobs, completedCount)
    }
}

func hasCompletedRun(t *testing.T, jobID string) bool {
    iter := scyllaClient.Session.Query(`SELECT status FROM job_runs WHERE job_id = ? ALLOW FILTERING`, jobID).Iter()
    defer iter.Close()
    
    if iter.NumRows() > 0 {
        return true
    }
    return false
}
