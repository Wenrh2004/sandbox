package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	
	"github.com/bytedance/sonic"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	
	"github.com/Wenrh2004/sandbox/pkg/cache/client"
	"github.com/Wenrh2004/sandbox/pkg/util/str"
)

var ErrMarshal = errors.New("[cache.MultiCache.Set] marshal error")

// MultiCache defines the methods for a multi-level cache
type MultiCache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, expire time.Duration) error
	Del(ctx context.Context, key string) error
	GetAndSet(ctx context.Context, key string, expire time.Duration, fn func() (T, error)) (T, error)
	GetAndSingleSet(ctx context.Context, key string, expire time.Duration, fn func() (T, error)) (T, error)
}

// multiCache implements the MultiCache interface
type multiCache[T any] struct {
	cache  []client.Cache
	sf     singleflight.Group
	prefix string
	expire time.Duration
}

// buildKey generates a cache key with the prefix
func (m *multiCache[T]) buildKey(key string) string {
	return strings.ToUpper(fmt.Sprintf("%s:%s", m.prefix, str.RemoveSpace(key)))
}

// Get retrieves a value from the cache
func (m *multiCache[T]) Get(ctx context.Context, key string) (T, error) {
	var result T
	var errVals []error
	cacheKey := m.buildKey(key)
	
	for i, c := range m.cache {
		value, err := c.Get(ctx, cacheKey)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				continue
			}
			errVals = append(errVals, fmt.Errorf("[cache.MultiCache.Get] %s cache error: %w", c.GetName(), err))
			continue
		}
		
		if value == nil {
			continue
		}
		
		// 反序列化数据
		if err := sonic.Unmarshal(value, &result); err != nil {
			errVals = append(errVals, fmt.Errorf("[cache.MultiCache.Get] unmarshal from %s cache failed: %w", c.GetName(), err))
			continue
		}
		
		// 异步回填到更高优先级缓存
		if i > 0 {
			go func(ctx context.Context, cacheKey string, value []byte, caches []client.Cache) {
				for _, cache := range caches {
					// 忽略回填错误，不影响主流程
					_ = cache.Set(ctx, cacheKey, value, m.expire)
				}
			}(context.Background(), cacheKey, value, m.cache[:i])
		}
		
		return result, nil
	}
	
	if len(errVals) > 0 {
		return result, errors.Join(errVals...)
	}
	
	return result, client.ErrNotFound
}

// Set stores a value in all cache layers
func (m *multiCache[T]) Set(ctx context.Context, key string, value T, expire time.Duration) error {
	var errVals []error
	cacheKey := m.buildKey(key)
	
	cacheValue, err := sonic.Marshal(value)
	if err != nil {
		return errors.Join(ErrMarshal, err)
	}
	
	for _, c := range m.cache {
		if err := c.Set(ctx, cacheKey, cacheValue, expire); err != nil {
			errVals = append(errVals, fmt.Errorf("[cache.MultiCache.Set] %s cache error: %w", c.GetName(), err))
		}
	}
	
	if len(errVals) > 0 {
		return errors.Join(errVals...)
	}
	
	return nil
}

// Del deletes a value from all cache layers
func (m *multiCache[T]) Del(ctx context.Context, key string) error {
	var errVals []error
	cacheKey := m.buildKey(key)
	
	for _, c := range m.cache {
		if err := c.Del(ctx, cacheKey); err != nil {
			errVals = append(errVals, fmt.Errorf("[cache.MultiCache.Del] %s cache error: %w", c.GetName(), err))
		}
	}
	
	if len(errVals) > 0 {
		return errors.Join(errVals...)
	}
	
	return nil
}

// GetAndSet calls the function to fetch and store it
func (m *multiCache[T]) GetAndSet(ctx context.Context, key string, expire time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	
	// 执行回源函数
	value, err := fn()
	if err != nil {
		return zero, fmt.Errorf("[cache.MultiCache.fetchAndCache] source function error: %w", err)
	}
	
	// 设置缓存
	if err := m.Set(ctx, key, value, expire); err != nil {
		if errors.Is(err, ErrMarshal) {
			return zero, err
		}
		// 只记录错误但不中断返回，因为已经获取到了值
		// logger.Errorf("Failed to set cache: %v", err)
	}
	
	return value, nil
}

// GetAndSingleSet ensures only one concurrent call to fetch and store the value using singleflight
func (m *multiCache[T]) GetAndSingleSet(ctx context.Context, key string, expire time.Duration, fn func() (T, error)) (T, error) {
	// 使用 singleflight 执行回源
	cacheKey := m.buildKey(key)
	v, err, _ := m.sf.Do(cacheKey, func() (interface{}, error) {
		return m.GetAndSet(ctx, key, expire, fn)
	})
	
	return v, err
}

// NewMultiCache creates a new MultiCache instance
func NewMultiCache[T any](conf *viper.Viper, cache []client.Cache) MultiCache[T] {
	if len(cache) == 0 {
		panic("at least one cache implementation is required")
	}
	
	// 按优先级排序 (低优先级数值 = 更高优先级)
	sort.Slice(cache, func(i, j int) bool {
		return cache[i].GetPriority() < cache[j].GetPriority()
	})
	
	return &multiCache[T]{
		cache:  cache,
		prefix: conf.GetString("app.data.cache.prefix"),
		expire: time.Duration(conf.GetInt("app.data.cache.expire")) * time.Second,
	}
}
