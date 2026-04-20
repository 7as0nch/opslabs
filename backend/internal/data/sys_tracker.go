/* *
 * @Author: chengjiang
 * @Date: 2025-11-27 01:34:49
 * @Description: 埋点跟踪数据访问层
**/
package data

import (
	"context"

	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/internal/db"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/models/generator/query"
	"go.uber.org/zap"
)

type trackerRepo struct {
	db    db.DataRepo
	log   *zap.Logger
	query *query.Query
}

func NewTrackerRepo(db db.DataRepo, log *zap.Logger) base.TrackerRepo {
	return &trackerRepo{
		db:    db,
		log:   log,
		query: query.Use(db.GetDB()),
	}
}

// BatchCreate 批量创建埋点数据
func (r *trackerRepo) BatchCreate(ctx context.Context, trackers []*model.SysTracker) (int32, int32, error) {
	if len(trackers) == 0 {
		return 0, 0, nil
	}

	// 使用gorm的CreateInBatches方法批量创建
	result := r.db.GetDB().WithContext(ctx).CreateInBatches(trackers, 100)
	if result.Error != nil {
		r.log.Error("Batch create tracker failed", zap.Error(result.Error))
		return 0, int32(len(trackers)), result.Error
	}

	successCount := int32(result.RowsAffected)
	failedCount := int32(len(trackers)) - successCount

	return successCount, failedCount, nil
}

// List 分页查询埋点数据
func (r *trackerRepo) List(ctx context.Context, pageNum, pageSize int32, appId, deviceId string, userId int64, typ, pageUrl, startTime, endTime string) ([]*model.SysTracker, int64, error) {
	offset := (pageNum - 1) * pageSize

	// 构建查询
	db := r.db.GetDB().WithContext(ctx).Model(&model.SysTracker{})

	// 添加查询条件
	if appId != "" {
		db = db.Where("app_id = ?", appId)
	}
	if deviceId != "" {
		db = db.Where("device_id = ?", deviceId)
	}
	if userId != 0 {
		db = db.Where("user_id = ?", userId)
	}
	if typ != "" {
		db = db.Where("type = ?", typ)
	}
	if pageUrl != "" {
		db = db.Where("page_url LIKE ?", "%"+pageUrl+"%")
	}
	if startTime != "" {
		db = db.Where("created_at >= ?", startTime)
	}
	if endTime != "" {
		db = db.Where("created_at <= ?", endTime)
	}

	// 计算总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		r.log.Error("Count tracker failed", zap.Error(err))
		return nil, 0, err
	}

	// 分页查询
	var trackers []*model.SysTracker
	if err := db.Offset(int(offset)).Limit(int(pageSize)).Order("created_at DESC").Find(&trackers).Error; err != nil {
		r.log.Error("List tracker failed", zap.Error(err))
		return nil, 0, err
	}

	return trackers, total, nil
}
