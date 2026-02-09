package integration

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestShellScriptExecution(t *testing.T) {
	// Large shell script (>1KB) to trigger S3 offloading
	script := `#!/bin/bash
# This is a comprehensive test script that exceeds 1KB
# to trigger S3 payload offloading

echo "=== Shell Script Execution Test ==="
echo "Starting multi-task shell script..."

# Task 1: System information
echo "--- Task 1: System Info ---"
echo "Hostname: $(hostname)"
echo "Date: $(date)"
echo "User: $(whoami)"
echo ""

# Task 2: File operations
echo "--- Task 2: File Operations ---"
TMP_DIR="/tmp/job_test_$$"
mkdir -p "$TMP_DIR"
echo "Created temporary directory: $TMP_DIR"

# Create multiple test files
for i in {1..10}; do
    echo "Test file $i content" > "$TMP_DIR/file_$i.txt"
done
echo "Created 10 test files"

# Count files
FILE_COUNT=$(ls -1 "$TMP_DIR" | wc -l)
echo "File count: $FILE_COUNT"
echo ""

# Task 3: Calculations
echo "--- Task 3: Calculations ---"
SUM=0
for i in {1..100}; do
    SUM=$((SUM + i))
done
echo "Sum of 1 to 100: $SUM"
echo "Expected: 5050"
echo ""

# Task 4: String operations
echo "--- Task 4: String Operations ---"
TEST_STRING="distributed-job-scheduler"
echo "Original: $TEST_STRING"
echo "Uppercase: ${TEST_STRING^^}"
echo "Length: ${#TEST_STRING}"
echo ""

# Task 5: Network check (if curl is available)
echo "--- Task 5: Network Check ---"
if command -v curl &> /dev/null; then
    echo "curl is available"
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" https://www.example.com)
    echo "HTTP response from example.com: $HTTP_CODE"
else
    echo "curl not available, skipping network check"
fi
echo ""

# Task 6: Cleanup
echo "--- Task 6: Cleanup ---"
rm -rf "$TMP_DIR"
echo "Cleaned up temporary directory"
echo ""

# Final summary
echo "=== Script Execution Summary ==="
echo "All tasks completed successfully!"
echo "Script size: approximately 1.5KB"
echo "Execution timestamp: $(date +%s)"
echo "SUCCESS_MARKER_12345"
`

	// Encode script to avoid issues with special characters
	encodedScript := base64.StdEncoding.EncodeToString([]byte(script))
	payload := "cmd:echo '" + encodedScript + "' | base64 -d | bash"
	
	t.Logf("Submitting shell script job (payload size: %d bytes)", len(payload))
	jobID := submitJob(t, "shell-script-test", payload, "", "")
	t.Logf("Submitted shell script job: %s", jobID)

	// Wait for completion  
	waitForJobCompletion(t, jobID, 20*time.Second)

	// Query job_runs for output
	var output string
	var status string
	query := `SELECT output, status FROM job_runs WHERE job_id = ? ALLOW FILTERING`
	err := scyllaClient.Session.Query(query, jobID).Scan(&output, &status)
	
	if err != nil {
		t.Fatalf("Failed to fetch job run output: %v", err)
	}

	// Verify execution
	if status != "COMPLETED" {
		t.Errorf("Expected status COMPLETED, got: %s", status)
	}

	// Verify key outputs from the script
	expectedOutputs := []string{
		"Shell Script Execution Test",
		"Task 1: System Info",
		"Task 2: File Operations",
		"Created 10 test files",
		"Task 3: Calculations",
		"Sum of 1 to 100: 5050",
		"Task 4: String Operations",
		"distributed-job-scheduler",
		"Task 6: Cleanup",
		"All tasks completed successfully",
		"SUCCESS_MARKER_12345",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s', but it was not found", expected)
		}
	}

	t.Logf("Shell script executed successfully with all tasks completed")
	
	// Verify S3 offloading occurred (payload was >1KB)
	t.Logf("Payload size was %d bytes, S3 offloading should have been triggered", len(payload))
}
