package client

import (
	"context"
	"errors"
	"fmt"
	"time"
	
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// RedisCache is a client for the Redis library
type RedisCache struct {
	rdb      *redis.Client
	name     string
	priority int
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("[client.RedisCache.Get]: empty key not allowed")
	}
	
	val, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("[client.RedisCache.Get]: %w", err)
	}
	return val, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value []byte, expire time.Duration) error {
	if key == "" {
		return fmt.Errorf("[client.RedisCache.Set]: empty key not allowed")
	}
	
	if value == nil {
		return fmt.Errorf("[client.RedisCache.Set]: nil value not allowed")
	}
	
	// 确保有过期时间
	if expire <= 0 {
		expire = 1 * time.Hour // 默认1小时
	}
	
	err := r.rdb.Set(ctx, key, value, expire).Err()
	if err != nil {
		return fmt.Errorf("[client.RedisCache.Set]: %w", err)
	}
	return nil
}

func (r *RedisCache) Del(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("[client.RedisCache.Del]: empty key not allowed")
	}
	
	// 即使key不存在也不会返回错误
	err := r.rdb.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("[client.RedisCache.Del]: %w", err)
	}
	return nil
}

func (r *RedisCache) GetName() string {
	return r.name
}

func (r *RedisCache) GetPriority() int {
	return r.priority
}

// NewRedis creates a new Redis cache client
func NewRedis(conf *viper.Viper) *RedisCache {
	// 创建Redis选项
	options := &redis.Options{
		Addr:     conf.GetString("app.data.redis.addr"),
		Password: conf.GetString("app.data.redis.password"),
		DB:       conf.GetInt("app.data.redis.db"),
		
		// 连接池配置
		PoolSize:     conf.GetInt("app.data.redis.poolSize"),
		MinIdleConns: conf.GetInt("app.data.redis.minIdleConns"),
		
		// 连接超时
		DialTimeout:  time.Duration(conf.GetInt("app.data.redis.dialTimeout")) * time.Second,
		ReadTimeout:  time.Duration(conf.GetInt("app.data.redis.readTimeout")) * time.Second,
		WriteTimeout: time.Duration(conf.GetInt("app.data.redis.writeTimeout")) * time.Second,
	}
	
	// 设置默认值
	if options.PoolSize <= 0 {
		options.PoolSize = 10
	}
	if options.DialTimeout <= 0 {
		options.DialTimeout = 5 * time.Second
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = 3 * time.Second
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = 3 * time.Second
	}
	
	rdb := redis.NewClient(options)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// 测试连接
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Redis connection failed: %s", err.Error()))
	}
	
	return &RedisCache{
		rdb:      rdb,
		name:     "redis",
		priority: conf.GetInt("app.data.cache.redis.priority"),
	}
}
