package chaos

import (
    "fmt"
    "os"
    "testing"
    
    "distributed_job_scheduler/pkg/infra"
)

var (
    scyllaClient *infra.ScyllaClient
)

func TestMain(m *testing.M) {
    // Setup
    var err error
    scyllaHosts := []string{"localhost"} 
    scyllaClient, err = infra.NewScyllaClient(scyllaHosts, "scheduler")
    if err != nil {
        fmt.Printf("Failed to connect to Scylla (localhost:9042): %v\n", err)
        os.Exit(1)
    }

    // Run Tests
    code := m.Run()

    // Teardown
    scyllaClient.Close()
    os.Exit(code)
}
