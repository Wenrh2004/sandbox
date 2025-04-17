package client

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is used to indicate a cache miss
var ErrNotFound = errors.New("no cache found")

// Cache defines the methods for a cache client.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expire time.Duration) error
	Del(ctx context.Context, key string) error
	GetPriority() int
	GetName() string
}
