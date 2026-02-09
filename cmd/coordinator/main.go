package main

import (
    "context"
    "log"
    "os"
    "strings"
    "time"

    "distributed_job_scheduler/pkg/infra"
    "go.etcd.io/etcd/client/v3/concurrency"
)

var (
    etcdClient *infra.EtcdClient
)

func main() {
    log.Println("Starting Coordinator Service...")

    initInfra()
    defer closeInfra()

    runElectionLoop()
}

func initInfra() {
    var err error

    // Etcd
    endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
    if len(endpoints) == 0 { 
        endpoints = []string{"scheduler-etcd:2379"} 
    }
    etcdClient, err = infra.NewEtcdClient(endpoints)
    if err != nil {
        log.Fatalf("Failed to connect to Etcd: %v", err)
    }
    log.Println("Connected to Etcd")
}

func closeInfra() {
    if etcdClient != nil { etcdClient.Close() }
}

func runElectionLoop() {
    // Create a session for concurrency (TTL 5s)
    session, err := concurrency.NewSession(etcdClient.Client, concurrency.WithTTL(5))
    if err != nil {
        log.Fatalf("Failed to create Etcd session: %v", err)
    }
    defer session.Close()

    e := concurrency.NewElection(session, "/scheduler/leader")

    // Campaign for leadership
    log.Println("Campaigning for leadership...")
    ctx := context.Background()
    
    // Campaign blocks until elected
    if err := e.Campaign(ctx, "coordinator-1"); err != nil {
        log.Fatalf("Campaign failed: %v", err)
    }

    log.Println("Elected as Leader!")
    
    // Leader Loop
    leaderCtx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Watch for connection loss or session expiry
    go func() {
        <-session.Done()
        log.Println("Etcd session expired, stepping down...")
        cancel()
        os.Exit(1) // simpler to restart pod
    }()

    balanceShards(leaderCtx)
}

func balanceShards(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // TODO: Watch active pickers and assign shards
            // For Phase 4.2 core, just log we are alive
            log.Println("Leader active. Checking cluster state...")
            
            // Logic to be implemented in full Phase 4.2
            // 1. Get Pickers from /scheduler/pickers/ prefix
            // 2. Distribute 0-1024 shards among them
            // 3. Write assignments to /scheduler/assignments/
        }
    }
}
