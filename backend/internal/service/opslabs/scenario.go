/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Scenario 服务层:场景注册表 -> proto 响应的纯转换
**/
package opslabs

import (
	"context"
	"errors"

	pb "github.com/7as0nch/backend/api/opslabs/v1"
	"github.com/7as0nch/backend/internal/scenario"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

// ScenarioService 场景元信息服务
type ScenarioService struct {
	pb.UnimplementedScenarioServer
	registry scenario.Registry
}

// NewScenarioService 构造
func NewScenarioService(registry scenario.Registry) *ScenarioService {
	return &ScenarioService{registry: registry}
}

// ListScenarios 列出所有场景
// Week 1 过滤参数先不生效,直接返回全量(已在 registry 内按难度升序)
func (s *ScenarioService) ListScenarios(ctx context.Context, req *pb.ListScenariosRequest) (*pb.ListScenariosReply, error) {
	list := s.registry.List()
	reply := &pb.ListScenariosReply{
		Scenarios: make([]*pb.ScenarioBrief, 0, len(list)),
		Total:     uint32(len(list)),
	}
	for _, sc := range list {
		reply.Scenarios = append(reply.Scenarios, toScenarioBrief(sc))
	}
	return reply, nil
}

// GetScenario 场景详情
func (s *ScenarioService) GetScenario(ctx context.Context, req *pb.GetScenarioRequest) (*pb.ScenarioReply, error) {
	if req.GetSlug() == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "slug is empty")
	}
	sc, err := s.registry.Get(req.GetSlug())
	if err != nil {
		if errors.Is(err, scenario.ErrScenarioNotFound) {
			return nil, kerrors.NotFound("SCENARIO_NOT_FOUND", "scenario not found: "+req.GetSlug())
		}
		return nil, kerrors.InternalServer("UNKNOWN", err.Error())
	}
	return &pb.ScenarioReply{Scenario: toScenarioDetail(sc)}, nil
}

// toScenarioBrief scenario.Scenario -> pb.ScenarioBrief
func toScenarioBrief(sc *scenario.Scenario) *pb.ScenarioBrief {
	return &pb.ScenarioBrief{
		Slug:             sc.Slug,
		Title:            sc.Title,
		Summary:          sc.Summary,
		Category:         sc.Category,
		Difficulty:       sc.Difficulty,
		EstimatedMinutes: sc.EstimatedMinutes,
		TargetPersonas:   sc.TargetPersonas,
		TechStack:        sc.TechStack,
		Tags:             sc.Tags,
		IsPremium:        sc.IsPremium,
		ExecutionMode:    sc.EffectiveExecutionMode(),
	}
}

// toScenarioDetail scenario.Scenario -> pb.ScenarioDetail
// Week 1 hint.content 始终置空,后续解锁逻辑上线后才下发
// bundleUrl 非 sandbox 模式下才下发,sandbox 为空串
func toScenarioDetail(sc *scenario.Scenario) *pb.ScenarioDetail {
	hints := make([]*pb.Hint, 0, len(sc.Hints))
	for _, h := range sc.Hints {
		hints = append(hints, &pb.Hint{
			Level:    h.Level,
			Unlocked: false,
			Content:  h.Content, // 不下发
		})
	}
	mode := sc.EffectiveExecutionMode()
	return &pb.ScenarioDetail{
		Slug:             sc.Slug,
		Version:          sc.Version,
		Title:            sc.Title,
		Summary:          sc.Summary,
		DescriptionMd:    sc.DescriptionMd,
		Category:         sc.Category,
		Difficulty:       sc.Difficulty,
		EstimatedMinutes: sc.EstimatedMinutes,
		TargetPersonas:   sc.TargetPersonas,
		ExperienceLevel:  sc.ExperienceLevel,
		TechStack:        sc.TechStack,
		Skills:           sc.Skills,
		Commands:         sc.Commands,
		Tags:             sc.Tags,
		Hints:            hints,
		IsPremium:        sc.IsPremium,
		ExecutionMode:    mode,
		BundleUrl:        bundleURLForMode(mode, sc.Slug),
	}
}

// bundleURLForMode 只有非 sandbox 模式返回 bundle 入口 URL
//
// sandbox 不需要 bundle,前端直连 terminal_url 起 ttyd iframe;
// static / wasm-linux / web-container 三种都需要拉前端资源,但入口文件名各不相同
// (static / wasm-linux 是 index.html;web-container 是 project.json),
// 具体映射收敛到 BundleEntryURLFor
func bundleURLForMode(mode, slug string) string {
	if mode == "" || mode == scenario.ExecutionModeSandbox {
		return ""
	}
	return BundleEntryURLFor(mode, slug)
}
