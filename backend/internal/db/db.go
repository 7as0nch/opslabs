/* *
 * @Author: chengjiang
 * @Date: 2025-12-11 15:48:17
 * @Description:
**/
package db

import (
	"context"

	"github.com/example/aichat/backend/internal/conf"
	"go.uber.org/zap"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DataRepo interface {
	GetDB() *gorm.DB
	DB(ctx context.Context) *gorm.DB
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	Redis() RedisRepo
	// TODO OSS
}

// Data .
type Data struct {
	// TODO wrapped database client
	db    *gorm.DB
	redis RedisRepo
}

// GetDB implements DataRepo.
func (d *Data) GetDB() *gorm.DB {
	return d.db
}

// DB implements DataRepo.
func (d *Data) DB(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(contextTxKey{}).(*gorm.DB)
	if ok {
		return tx.WithContext(ctx)
	}
	return d.db.WithContext(ctx)
}

type contextTxKey struct{}

// InTx implements DataRepo.
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = context.WithValue(ctx, contextTxKey{}, tx)
		err := fn(ctx)
		if err != nil {
			log.Errorf("Transaction rollback: %v", err)
			return err
		}
		log.Info("Transaction commit")
		return nil
	})
}

// Redis implements DataRepo.
func (d *Data) Redis() RedisRepo {
	return d.redis
}

// NewData .
func NewData(c *conf.Bootstrap, logger *zap.Logger) (DataRepo, func(), error) {
	// 初始化mysqldb
	db := MustNewPostgresDB(c, logger)
	if db == nil {
		logger.Error("NewMysqlDB failed")
		return nil, nil, nil
	}
	redisRepo, err := NewRedis(c, logger)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		logger.Info("closing the data resources")
		if redisRepo != nil {
			_ = redisRepo.Close()
		}
	}
	return &Data{
		db:    db,
		redis: redisRepo,
	}, cleanup, nil
}

func NewMysqlDB(source string) *gorm.DB {
	db, err := gorm.Open(mysql.Open(source), &gorm.Config{})
	if err != nil {
		log.Error(err)
		panic(err)
	}
	return db
}

func NewRedisRepo(d DataRepo) RedisRepo {
	return d.Redis()
}

