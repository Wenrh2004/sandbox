package client

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"
	
	"github.com/coocood/freecache"
	"github.com/spf13/viper"
)

// LocalCache is a client for the free cache library
type LocalCache struct {
	cache    *freecache.Cache
	name     string
	priority int
}

func (l *LocalCache) Get(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("[client.LocalCache.Get]: empty key not allowed")
	}
	
	bytes, err := l.cache.Get([]byte(key))
	if err != nil {
		if errors.Is(err, freecache.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("[client.LocalCache.Get]: %w", err)
	}
	return bytes, nil
}

func (l *LocalCache) Set(ctx context.Context, key string, value []byte, expire time.Duration) error {
	if key == "" {
		return fmt.Errorf("[client.LocalCache.Set]: empty key not allowed")
	}
	
	if value == nil {
		return fmt.Errorf("[client.LocalCache.Set]: nil value not allowed")
	}
	
	// 防止超出 freecache 的 key 和 value 长度限制
	if len(key) > 65535 {
		return fmt.Errorf("[client.LocalCache.Set]: key too long (max 65535 bytes)")
	}
	
	if len(value) > 1<<30 { // 1GB
		return fmt.Errorf("[client.LocalCache.Set]: value too large (max 1GB)")
	}
	
	// 确保过期时间不超过 freecache 最大值
	expireSeconds := int(expire.Seconds())
	if expireSeconds <= 0 {
		expireSeconds = 60 // 默认1分钟，防止永久缓存
	}
	
	err := l.cache.Set([]byte(key), value, expireSeconds)
	if err != nil {
		return fmt.Errorf("[client.LocalCache.Set]: %w", err)
	}
	return nil
}

func (l *LocalCache) Del(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("[client.LocalCache.Del]: empty key not allowed")
	}
	
	if ok := l.cache.Del([]byte(key)); !ok {
		// 这里不返回错误，因为可能是键不存在
		return nil
	}
	return nil
}

func (l *LocalCache) GetPriority() int {
	return l.priority
}

func (l *LocalCache) GetName() string {
	return l.name
}

// NewLocalCache creates a new LocalCache instance
func NewLocalCache(conf *viper.Viper) *LocalCache {
	size := conf.GetInt("app.data.cache.local.size")
	if size <= 0 {
		size = 100 // 默认 100 MB
	}
	
	// 创建缓存，大小单位为 MB
	c := freecache.NewCache(size * 1024 * 1024)
	
	// 设置GC百分比
	gcPercent := conf.GetInt("app.data.cache.local.gcPercent")
	if gcPercent > 0 {
		debug.SetGCPercent(gcPercent)
	}
	
	return &LocalCache{
		cache:    c,
		name:     "local",
		priority: conf.GetInt("app.data.cache.local.priority"),
	}
}
