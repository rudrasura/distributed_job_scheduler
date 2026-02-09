package infra

import (
    "time"

    "github.com/gocql/gocql"
)

type ScyllaClient struct {
    Session *gocql.Session
}

func NewScyllaClient(hosts []string, keyspace string) (*ScyllaClient, error) {
    cluster := gocql.NewCluster(hosts...)
    cluster.Keyspace = keyspace
    cluster.Consistency = gocql.Quorum
    cluster.Timeout = 5 * time.Second
    cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: 3}
    cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

    session, err := cluster.CreateSession()
    if err != nil {
        return nil, err
    }

    return &ScyllaClient{Session: session}, nil
}

func (s *ScyllaClient) Close() {
    if s.Session != nil {
        s.Session.Close()
    }
}
