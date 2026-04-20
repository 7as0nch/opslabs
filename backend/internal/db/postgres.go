package db

import (
	"fmt"
	"time"

	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/models"
	"github.com/example/aichat/backend/pkg/auth"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// GormZapWriter 实现 gorm 日志接口的 zap writer 适配器
type GormZapWriter struct {
	log *zap.Logger
}

// Printf 实现 gorm logger.Writer 接口
func (w GormZapWriter) Printf(format string, args ...interface{}) {
	// 使用 Info 级别记录 SQL 日志
	w.log.Info(fmt.Sprintf(format, args...))
}

// NewPostgresDB 创建 PostgreSQL 数据库连接，支持 SQL 日志打印
func NewPostgresDB(conf *conf.Bootstrap, log *zap.Logger) (*gorm.DB, error) {
	pgConfig := conf.Data.PgDatabase
	// 构建 DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=aichat",
		pgConfig.Host,
		pgConfig.Port,
		pgConfig.User,
		pgConfig.Password,
		pgConfig.Dbname,
		pgConfig.Sslmode,
	)

	// 自定义 GORM 日志配置，支持 SQL 日志打印
	newLogger := logger.New(
		GormZapWriter{log: log},
		logger.Config{
			SlowThreshold:             time.Second, // 慢 SQL 阈值
			LogLevel:                  logger.Info, // 日志级别，控制 SQL 日志的详细程度
			IgnoreRecordNotFoundError: true,        // 忽略记录未找到错误
			Colorful:                  true,        // 彩色打印，增强可读性
		},
	)

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: newLogger, // 启用 SQL 日志
	}

	// 连接数据库
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	// before create 钩子，自动设置 created_at created_by 字段
	db.Callback().Create().Before("gorm:create").Register("before_create", beforeCreate)

	// before update 钩子，自动设置 updated_at updated_by 字段
	db.Callback().Update().Before("gorm:update").Register("before_update", beforeUpdate)

	// 获取标准 Delete 回调
	defaultDeleteCallback := db.Callback().Delete().Get("gorm:delete")

	// 替换标准 Delete 回调，实现自定义软删除逻辑
	db.Callback().Delete().Replace("gorm:delete", func(db *gorm.DB) {
		customDelete(db, defaultDeleteCallback)
	})

	// 配置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置连接池参数，优化数据库性能
	sqlDB.SetMaxIdleConns(10)           // 最大空闲连接数
	sqlDB.SetMaxOpenConns(100)          // 最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生命周期

	// 记录连接信息
	log.Info("PostgreSQL connection established",
		zap.String("host", pgConfig.Host),
		zap.Int("port", int(pgConfig.Port)),
		zap.String("dbname", pgConfig.Dbname),
	)

	return db, nil
}

// MustNewPostgresDB 创建 PostgreSQL 数据库连接，如果出错则 panic
// 适用于应用启动时的数据库初始化，确保数据库连接成功
func MustNewPostgresDB(conf *conf.Bootstrap, log *zap.Logger) *gorm.DB {
	db, err := NewPostgresDB(conf, log)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to PostgreSQL: %v", err))
	}
	return db
}

func beforeCreate(db *gorm.DB) {
	// 从context中获取当前用户ID
	ctx := db.Statement.Context
	var userID int64 = 0 // 默认系统操作

	// 检查是否有可用的用户ID（从认证middleware或业务逻辑中获取）
	if ctx != nil {
		if authUserID := ctx.Value(auth.UserId); authUserID != nil {
			if uid, ok := authUserID.(int64); ok {
				userID = uid
			} else if uid, ok := authUserID.(int); ok {
				userID = int64(uid)
			}
		}
	}

	// 设置创建时间和操作用户ID
	now := models.Now()

	// 直接设置字段，让GORM自动处理字段覆盖问题
	db.Statement.SetColumn("created_at", now)
	db.Statement.SetColumn("created_by", userID)
}

func beforeUpdate(db *gorm.DB) {
	// 从context中获取当前用户ID
	ctx := db.Statement.Context
	var userID int64 = 0 // 默认系统操作

	// 检查是否有可用的用户ID（从认证middleware或业务逻辑中获取）
	if ctx != nil {
		if authUserID := ctx.Value(auth.UserId); authUserID != nil {
			if uid, ok := authUserID.(int64); ok {
				userID = uid
			} else if uid, ok := authUserID.(int); ok {
				userID = int64(uid)
			}
		}
	}

	// 设置更新时间和操作用户ID
	now := models.Now()

	// 直接设置字段，让GORM自动处理字段覆盖问题
	db.Statement.SetColumn("updated_at", now)
	db.Statement.SetColumn("updated_by", userID)
}

func customDelete(db *gorm.DB, fallback func(*gorm.DB)) {
	if db.Error != nil {
		return
	}

	if db.Statement.Schema != nil {
		// 检查是否存在 IsDeleted 字段，如果存在则执行自定义软删除
		if db.Statement.Schema.LookUpField("IsDeleted") != nil {
			// 从context中获取当前用户ID
			ctx := db.Statement.Context
			var userID int64 = 0 // 默认系统操作

			if ctx != nil {
				if authUserID := ctx.Value(auth.UserId); authUserID != nil {
					if uid, ok := authUserID.(int64); ok {
						userID = uid
					} else if uid, ok := authUserID.(int); ok {
						userID = int64(uid)
					}
				}
			}

			now := models.Now()

			// 构建 UPDATE 语句
			// 设置 is_deleted = 1
			// 设置 deleted_at, deleted_by
			db.Statement.AddClause(clause.Set{
				{Column: clause.Column{Name: "is_deleted"}, Value: 1},
				{Column: clause.Column{Name: "deleted_at"}, Value: now},
				{Column: clause.Column{Name: "deleted_by"}, Value: userID},
			})
			db.Statement.AddClause(clause.Update{})

			// 构建并执行 SQL
			db.Statement.Build("UPDATE", "SET", "WHERE")
			if _, err := db.Statement.ConnPool.ExecContext(db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...); err != nil {
				db.AddError(err)
			}
			return
		}
	}

	// 如果不是软删除模型，执行标准删除
	if fallback != nil {
		fallback(db)
	}
}
