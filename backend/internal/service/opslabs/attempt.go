/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Attempt 服务层:proto <-> biz.AttemptUsecase 适配
**/
package opslabs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/7as0nch/backend/api/opslabs/v1"
	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/7as0nch/backend/models/generator/model"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

// AttemptService 场景尝试服务
type AttemptService struct {
	pb.UnimplementedAttemptServer
	uc   *attempt.AttemptUsecase
	opts *ServiceOptions
}

// NewAttemptService 构造
func NewAttemptService(uc *attempt.AttemptUsecase, opts *ServiceOptions) *AttemptService {
	if opts == nil {
		opts = DefaultServiceOptions()
	}
	return &AttemptService{uc: uc, opts: opts}
}

// StartScenario 启动场景
func (s *AttemptService) StartScenario(ctx context.Context, req *pb.StartScenarioRequest) (*pb.StartScenarioReply, error) {
	a, err := s.uc.Start(ctx, req.GetSlug())
	if err != nil {
		return nil, err
	}
	return &pb.StartScenarioReply{
		AttemptId:   strconv.FormatInt(a.ID, 10),
		TerminalUrl: s.terminalURL(a),
		ExpiresAt:   s.expiresAt(a).Format(time.RFC3339),
	}, nil
}

// GetAttempt 查询状态
func (s *AttemptService) GetAttempt(ctx context.Context, req *pb.GetAttemptRequest) (*pb.AttemptReply, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	a, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &pb.AttemptReply{
		AttemptId:    strconv.FormatInt(a.ID, 10),
		ScenarioSlug: a.ScenarioSlug,
		Status:       string(a.Status),
		TerminalUrl:  s.terminalURL(a),
		StartedAt:    a.StartedAt.Format(time.RFC3339),
		LastActiveAt: a.LastActiveAt.Format(time.RFC3339),
	}, nil
}

// CheckAttempt 判题
func (s *AttemptService) CheckAttempt(ctx context.Context, req *pb.CheckAttemptRequest) (*pb.CheckAttemptReply, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	res, err := s.uc.Check(ctx, id)
	if err != nil {
		return nil, err
	}
	reply := &pb.CheckAttemptReply{
		Passed:     res.Passed,
		CheckCount: uint32(res.Attempt.CheckCount),
	}
	if res.Passed {
		reply.Message = "恭喜通关"
		if res.Attempt.DurationMS != nil {
			reply.DurationSeconds = uint32(*res.Attempt.DurationMS / 1000)
		}
	} else {
		// 把 stderr/stdout 压缩成一行短消息,前端自行展开
		reply.Message = firstNonEmpty(res.Stderr, res.Stdout, "still not passing")
	}
	return reply, nil
}

// TerminateAttempt 主动结束
func (s *AttemptService) TerminateAttempt(ctx context.Context, req *pb.TerminateAttemptRequest) (*pb.TerminateAttemptReply, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.uc.Terminate(ctx, id); err != nil {
		return nil, err
	}
	a, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &pb.TerminateAttemptReply{Status: string(a.Status)}, nil
}

// ==============================================================
// 内部工具
// ==============================================================

// terminalURL 按模板渲染 ttyd 访问地址
//
// HostPort==0 约定为"没有真实终端"(例如 mock runtime),此时返回空串,
// 前端据此不去 iframe 那个必然失败的 URL,改为渲染"预览模式"提示卡。
func (s *AttemptService) terminalURL(a *model.OpslabsAttempt) string {
	if a.HostPort <= 0 {
		return ""
	}
	host := s.opts.TerminalHost
	tmpl := s.opts.TerminalURLTemplate
	if host == "" {
		host = "localhost"
	}
	if tmpl == "" {
		tmpl = "http://{host}:{port}/"
	}
	url := strings.ReplaceAll(tmpl, "{host}", host)
	url = strings.ReplaceAll(url, "{port}", strconv.Itoa(a.HostPort))
	return url
}

// expiresAt LastActiveAt + 默认空闲超时
func (s *AttemptService) expiresAt(a *model.OpslabsAttempt) time.Time {
	idle := s.opts.DefaultIdleTimeout
	if idle <= 0 {
		idle = 30 * time.Minute
	}
	return a.LastActiveAt.Add(idle)
}

// parseID "123" -> 123
func parseID(s string) (int64, error) {
	if s == "" {
		return 0, kerrors.BadRequest("INVALID_ARGUMENT", "id is empty")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, kerrors.BadRequest("INVALID_ARGUMENT", fmt.Sprintf("invalid id: %s", s))
	}
	return v, nil
}

// firstNonEmpty 按顺序取第一个非空字符串
func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}
