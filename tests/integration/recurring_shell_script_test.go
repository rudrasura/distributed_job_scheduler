package integration

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestRecurringShellScript(t *testing.T) {
	// Shell script for recurring execution
	script := `#!/bin/bash
# Recurring job test script
echo "=== Recurring Shell Script Execution ==="
echo "Execution timestamp: $(date +%s)"
echo "Hostname: $(hostname)"

# Create a unique marker with timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
echo "Run ID: RUN_$TIMESTAMP"

# Perform some work
RANDOM_NUM=$((RANDOM % 1000))
echo "Random number: $RANDOM_NUM"

# Calculate something
RESULT=$((RANDOM_NUM * 2))
echo "Result (x2): $RESULT"

echo "Recurring job execution completed"
echo "RECURRING_SUCCESS_MARKER"
`

	encodedScript := base64.StdEncoding.EncodeToString([]byte(script))
	payload := "cmd:echo '" + encodedScript + "' | base64 -d | bash"
	
	// Submit recurring job that runs every 3 seconds
	cronSchedule := "*/3 * * * * *" // Every 3 seconds
	scheduledAt := time.Now().Add(1 * time.Second).Format(time.RFC3339) // Start in 1 second
	
	t.Logf("Submitting recurring shell script job (cron: %s)", cronSchedule)
	jobID := submitJob(t, "recurring-script-test", payload, cronSchedule, scheduledAt)
	t.Logf("Submitted recurring job: %s", jobID)

	// Wait for first execution
	t.Log("Waiting for first execution...")
	time.Sleep(5 * time.Second)

	// Query first run
	var firstOutput string
	var firstStatus string
	query := `SELECT output, status FROM job_runs WHERE job_id = ? LIMIT 1 ALLOW FILTERING`
	err := scyllaClient.Session.Query(query, jobID).Scan(&firstOutput, &firstStatus)
	
	if err != nil {
		t.Fatalf("Failed to fetch first run output: %v", err)
	}

	if firstStatus != "COMPLETED" {
		t.Errorf("First run status: expected COMPLETED, got %s", firstStatus)
	}

	if !strings.Contains(firstOutput, "RECURRING_SUCCESS_MARKER") {
		t.Errorf("First run output doesn't contain success marker")
	}

	t.Log("First execution completed successfully")

	// Wait for recurrence (at least 3 more seconds)
	t.Log("Waiting for second execution (recurrence interval: 3s)...")
	time.Sleep(5 * time.Second)

	// Query all runs
	type JobRun struct {
		Output string
		Status string
	}
	
	var runs []JobRun
	iter := scyllaClient.Session.Query(`SELECT output, status FROM job_runs WHERE job_id = ? ALLOW FILTERING`, jobID).Iter()
	
	var output, status string
	for iter.Scan(&output, &status) {
		runs = append(runs, JobRun{Output: output, Status: status})
	}
	
	if err := iter.Close(); err != nil {
		t.Fatalf("Failed to query job runs: %v", err)
	}

	// Verify at least 2 executions occurred
	if len(runs) < 2 {
		t.Errorf("Expected at least 2 executions, got %d", len(runs))
	} else {
		t.Logf("Verified %d executions of recurring job", len(runs))
		
		// Verify all runs completed successfully
		for i, run := range runs {
			if run.Status != "COMPLETED" {
				t.Errorf("Run %d: expected COMPLETED, got %s", i+1, run.Status)
			}
			if !strings.Contains(run.Output, "RECURRING_SUCCESS_MARKER") {
				t.Errorf("Run %d: output missing success marker", i+1)
			}
		}
	}

	// Verify job is still scheduled for next run
	var nextFireAt time.Time
	err = scyllaClient.Session.Query(`SELECT next_fire_at FROM jobs WHERE job_id = ?`, jobID).Scan(&nextFireAt)
	if err != nil {
		t.Fatalf("Failed to query next_fire_at: %v", err)
	}

	if nextFireAt.Before(time.Now()) {
		t.Errorf("next_fire_at is in the past: %v", nextFireAt)
	} else {
		t.Logf("Job scheduled for next execution at: %v", nextFireAt)
	}

	t.Log("Recurring shell script test completed successfully")
}
