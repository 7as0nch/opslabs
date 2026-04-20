/* *
 * @Author: chengjiang
 * @Date: 2025-11-27 01:34:49
 * @Description: 埋点跟踪业务逻辑层
**/
package base

import (
	"context"

	"github.com/example/aichat/backend/models/generator/model"
)

type TrackerRepo interface {
	// BatchCreate 批量创建埋点数据
	BatchCreate(ctx context.Context, trackers []*model.SysTracker) (int32, int32, error)
	// List 分页查询埋点数据
	List(ctx context.Context, pageNum, pageSize int32, appId, deviceId string, userId int64, typ, pageUrl, startTime, endTime string) ([]*model.SysTracker, int64, error)
}

type TrackerUseCase struct {
	repo TrackerRepo
}

func NewTrackerUseCase(repo TrackerRepo) *TrackerUseCase {
	return &TrackerUseCase{
		repo: repo,
	}
}

// BatchCreate 批量创建埋点数据
func (uc *TrackerUseCase) BatchCreate(ctx context.Context, trackers []*model.SysTracker) (int32, int32, error) {
	return uc.repo.BatchCreate(ctx, trackers)
}

// List 分页查询埋点数据
func (uc *TrackerUseCase) List(ctx context.Context, pageNum, pageSize int32, appId, deviceId string, userId int64, typ, pageUrl, startTime, endTime string) ([]*model.SysTracker, int64, error) {
	return uc.repo.List(ctx, pageNum, pageSize, appId, deviceId, userId, typ, pageUrl, startTime, endTime)
}
