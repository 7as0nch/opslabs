/* *
 * @Author: chengjiang
 * @Date: 2026-03-16 16:48:00
 * @Description:
**/
package db

import (
	"context"
	"time"

	"github.com/example/aichat/backend/internal/conf"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, key string) error
	GetDel(ctx context.Context, key string) (string, error)
	RPush(ctx context.Context, key string, expiration time.Duration, values ...interface{}) error
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	Close() error
}

type redisRepo struct {
	client *redis.Client
}

func NewRedis(c *conf.Bootstrap, logger *zap.Logger) (RedisRepo, error) {
	if c == nil || c.GetData() == nil || c.GetData().GetRedis() == nil {
		return nil, nil
	}

	rc := c.GetData().GetRedis()
	readTimeout := time.Duration(0)
	writeTimeout := time.Duration(0)
	if rc.GetReadTimeout() != nil {
		readTimeout = rc.GetReadTimeout().AsDuration()
	}
	if rc.GetWriteTimeout() != nil {
		writeTimeout = rc.GetWriteTimeout().AsDuration()
	}
	client := redis.NewClient(&redis.Options{
		Addr:         rc.GetAddr(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Password:     rc.GetPassword(),
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		logger.Error("redis ping failed", zap.Error(err))
		return nil, err
	}
	return &redisRepo{client: client}, nil
}

func (r *redisRepo) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *redisRepo) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *redisRepo) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *redisRepo) GetDel(ctx context.Context, key string) (string, error) {
	return r.client.GetDel(ctx, key).Result()
}

func (r *redisRepo) RPush(ctx context.Context, key string, expiration time.Duration, values ...interface{}) error {
	pipe := r.client.TxPipeline()
	pipe.RPush(ctx, key, values...)
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisRepo) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

func (r *redisRepo) Close() error {
	return r.client.Close()
}