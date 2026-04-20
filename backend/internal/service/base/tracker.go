/* *
 * @Author: chengjiang
 * @Date: 2025-11-27 01:34:49
 * @Description: 埋点跟踪服务层
**/
package base

import (
	"context"

	pb "github.com/example/aichat/backend/api/base"
	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/models/generator/model"
)

type TrackerService struct {
	pb.UnimplementedTrackerServer
	tracker *base.TrackerUseCase
}

func NewTrackerService(tracker *base.TrackerUseCase) *TrackerService {
	return &TrackerService{
		tracker: tracker,
	}
}

// Batch 批量新增埋点数据
func (s *TrackerService) Batch(ctx context.Context, req *pb.BatchRequest) (*pb.BatchReply, error) {
	// 将proto消息转换为model
	var trackers []*model.SysTracker
	for _, t := range req.Logs {
		temp := &model.SysTracker{
			AppId:     t.AppId,
			DeviceId:  t.DeviceId,
			UserId:    t.UserId,
			Timestamp: t.Timestamp,
			UserAgent: t.UserAgent,
			PageUrl:   t.PageUrl,
			Type:      model.TrackerType(t.Type),
			Data:      t.Data,
		}
		temp.New()
		trackers = append(trackers, temp)
	}

	// 调用biz层批量创建
	successCount, failedCount, err := s.tracker.BatchCreate(ctx, trackers)
	if err != nil {
		return nil, err
	}

	return &pb.BatchReply{
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

// List 分页查询埋点数据
func (s *TrackerService) List(ctx context.Context, req *pb.ListRequest) (*pb.ListReply, error) {
	// 调用biz层查询
	trackers, total, err := s.tracker.List(
		ctx,
		req.PageNum,
		req.PageSize,
		req.AppId,
		req.DeviceId,
		req.UserId,
		req.Type,
		req.PageUrl,
		req.StartTime,
		req.EndTime,
	)
	if err != nil {
		return nil, err
	}

	// 将model转换为proto消息
	var pbTrackers []*pb.TrackerMessage
	for _, t := range trackers {
		pbTrackers = append(pbTrackers, &pb.TrackerMessage{
			AppId:     t.AppId,
			DeviceId:  t.DeviceId,
			UserId:    t.UserId,
			Timestamp: t.Timestamp,
			UserAgent: t.UserAgent,
			PageUrl:   t.PageUrl,
			Type:      string(t.Type),
			Data:      t.Data,
		})
	}

	return &pb.ListReply{
		List:  pbTrackers,
		Total: int32(total),
	}, nil
}
