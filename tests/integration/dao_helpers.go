package integration

import (
    "testing"
    "time"
)

type JobDAO struct {
    JobID        string
    NextFireAt   time.Time
    Status       string
    CronSchedule string
}

func getJobDetails(t *testing.T, jobID string) JobDAO {
    var j JobDAO
    // Fetch relevant fields
    query := `SELECT job_id, next_fire_at, status, cron_schedule FROM jobs WHERE job_id = ?`
    err := scyllaClient.Session.Query(query, jobID).Scan(&j.JobID, &j.NextFireAt, &j.Status, &j.CronSchedule)
    if err != nil {
        t.Fatalf("Failed to fetch job details for %s: %v", jobID, err)
    }
    return j
}
