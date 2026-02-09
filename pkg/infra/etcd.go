package infra

import (
    "time"

    clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
    Client *clientv3.Client
}

func NewEtcdClient(endpoints []string) (*EtcdClient, error) {
    cli, err := clientv3.New(clientv3.Config{
        Endpoints:   endpoints,
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        return nil, err
    }

    return &EtcdClient{Client: cli}, nil
}

func (e *EtcdClient) Close() error {
    return e.Client.Close()
}
