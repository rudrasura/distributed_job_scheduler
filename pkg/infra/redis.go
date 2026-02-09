package infra

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
)

type RedisClient struct {
    Client *redis.Client
}

func NewRedisClient(addr string, password string, db int) (*RedisClient, error) {
    rdb := redis.NewClient(&redis.Options{
        Addr:         addr,
        Password:     password,
        DB:           db,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolSize:     10,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, err
    }

    return &RedisClient{Client: rdb}, nil
}

func (r *RedisClient) Close() error {
    return r.Client.Close()
}
