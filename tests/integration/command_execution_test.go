package integration

import (
	"strings"
	"testing"
	"time"
)

func TestCommandExecution(t *testing.T) {
	// Submit job with curl command
	jobID := submitJob(t, "cmd-execution-test", "cmd:curl -s https://www.example.com/", "", "")
	t.Logf("Submitted command execution job: %s", jobID)

	// Wait for completion
	waitForJobCompletion(t, jobID, 15*time.Second)

	// Query job_runs table for output
	var output string
	query := `SELECT output FROM job_runs WHERE job_id = ? ALLOW FILTERING`
	err := scyllaClient.Session.Query(query, jobID).Scan(&output)
	
	if err != nil {
		t.Fatalf("Failed to fetch job run output: %v", err)
	}

	// Verify output contains HTML from example.com
	if !strings.Contains(output, "Example Domain") {
		t.Errorf("Expected output to contain 'Example Domain', got: %s", output[:min(len(output), 200)])
	}

	// Verify output contains typical HTML elements
	if !strings.Contains(output, "<title>") && !strings.Contains(output, "<html") {
		t.Errorf("Expected output to contain HTML tags, got: %s", output[:min(len(output), 200)])
	}

	t.Log("Command execution verified: curl successfully fetched example.com")
}
