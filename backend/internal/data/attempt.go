/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Attempt 数据访问层,走 gorm(后续 gen 生成的 query 也可直接复用)
**/
package data

import (
	"context"
	"errors"

	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/7as0nch/backend/internal/db"
	"github.com/7as0nch/backend/models/generator/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type attemptRepo struct {
	db  db.DataRepo
	log *zap.Logger
}

// NewAttemptRepo 创建 Attempt Repo
//
// 注:表结构由 backend/models/generator/db_test.go::TestMigrate 统一建表,
// 这里不做 AutoMigrate,保持和 SysTracker 等其他 repo 一致
func NewAttemptRepo(d db.DataRepo, log *zap.Logger) attempt.AttemptRepo {
	return &attemptRepo{db: d, log: log}
}

// Create 新建一条 Attempt 记录
func (r *attemptRepo) Create(ctx context.Context, a *attempt.Attempt) error {
	if err := r.db.GetDB().WithContext(ctx).Create(a).Error; err != nil {
		r.log.Error("create attempt failed", zap.Error(err), zap.Int64("id", a.ID))
		return err
	}
	return nil
}

// Update 更新整条 Attempt(走 Save,全字段刷新)
func (r *attemptRepo) Update(ctx context.Context, a *attempt.Attempt) error {
	if err := r.db.GetDB().WithContext(ctx).Save(a).Error; err != nil {
		r.log.Error("update attempt failed", zap.Error(err), zap.Int64("id", a.ID))
		return err
	}
	return nil
}

// FindByID 按主键查询,未找到时返回 ErrAttemptNotFound
func (r *attemptRepo) FindByID(ctx context.Context, id int64) (*attempt.Attempt, error) {
	var a model.OpslabsAttempt
	err := r.db.GetDB().WithContext(ctx).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, attempt.ErrAttemptNotFound
	}
	if err != nil {
		r.log.Error("find attempt failed", zap.Error(err), zap.Int64("id", id))
		return nil, err
	}
	return &a, nil
}

// 注:原 ListRunning(进程重启回灌内存缓存用)已于 Round 6 移除。
// AttemptStore 迁 Redis 后,跨进程共享状态,不再需要回灌机制。
