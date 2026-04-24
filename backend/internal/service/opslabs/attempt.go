/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Attempt 服务层:proto <-> biz.AttemptUsecase 适配
**/
package opslabs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	pb "github.com/7as0nch/backend/api/opslabs/v1"
	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/7as0nch/backend/internal/scenario"
	"github.com/7as0nch/backend/models/generator/model"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

// AttemptService 场景尝试服务
type AttemptService struct {
	pb.UnimplementedAttemptServer
	uc       *attempt.AttemptUsecase
	opts     *ServiceOptions
	registry scenario.Registry
}

// NewAttemptService 构造
//
// registry 用来在响应里填充 execution_mode / bundle_url 等运行时才知道的字段。
// 这两个字段没进 Attempt 的 DB schema —— 每次按 scenarioSlug 从 registry 反查,
// 避免重复落库 & 将来改场景元信息要跑 DB migration
func NewAttemptService(uc *attempt.AttemptUsecase, opts *ServiceOptions, registry scenario.Registry) *AttemptService {
	if opts == nil {
		opts = DefaultServiceOptions()
	}
	return &AttemptService{uc: uc, opts: opts, registry: registry}
}

// StartScenario 启动场景
func (s *AttemptService) StartScenario(ctx context.Context, req *pb.StartScenarioRequest) (*pb.StartScenarioReply, error) {
	a, err := s.uc.Start(ctx, req.GetSlug())
	if err != nil {
		return nil, err
	}
	mode, bundleURL := s.runtimeEntry(a.ScenarioSlug)
	return &pb.StartScenarioReply{
		AttemptId:     strconv.FormatInt(a.ID, 10),
		TerminalUrl:   s.terminalURL(a),
		ExpiresAt:     s.expiresAt(a).Format(time.RFC3339),
		ExecutionMode: mode,
		BundleUrl:     bundleURL,
	}, nil
}

// GetAttempt 查询状态
//
// 注:响应里没有 expires_at —— proto 未加该字段(避开 regen 依赖)。
// 前端持有 StartScenarioReply.expiresAt 作为基准,并按 GetAttempt 的 last_active_at
// 的位移做线性推断(last_active_at + 服务默认 idle_timeout),够用。
//
// 2026-04-22 增强(P3 fix):对 sandbox running attempt 预先 Ping,
// 容器已死返回 NEEDS_RESTART(409) —— 前端拦截后走 Start 新建,避免 iframe 指死端口。
//
// NEEDS_RESTART 语义(契约):
//   - 后端已经把内存 + DB 清成 terminated,这次请求不会再打出同一条 attempt
//   - 前端应当 toast 一条"会话已过期,正在重新启动",然后走 StartScenario 重新拉起
//   - 幂等:前端重复收到 NEEDS_RESTART 是合法的(连续两次 Get 都在 Ping 失败窗口)
func (s *AttemptService) GetAttempt(ctx context.Context, req *pb.GetAttemptRequest) (*pb.AttemptReply, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	a, alive, err := s.uc.GetWithPing(ctx, id)
	if err != nil {
		return nil, err
	}
	if !alive {
		// 容器已死,biz 层已自清。用 Conflict(409) 传达"资源状态和前端预期不符",
		// reason=NEEDS_RESTART 让前端无歧义地走 Start 新建路径
		return nil, kerrors.Conflict("NEEDS_RESTART",
			"sandbox container no longer alive; please start a new attempt")
	}
	mode, bundleURL := s.runtimeEntry(a.ScenarioSlug)
	return &pb.AttemptReply{
		AttemptId:     strconv.FormatInt(a.ID, 10),
		ScenarioSlug:  a.ScenarioSlug,
		Status:        string(a.Status),
		TerminalUrl:   s.terminalURL(a),
		StartedAt:     a.StartedAt.Format(time.RFC3339),
		LastActiveAt:  a.LastActiveAt.Format(time.RFC3339),
		ExecutionMode: mode,
		BundleUrl:     bundleURL,
	}, nil
}

// CheckAttempt 判题
//
// 非 sandbox 模式下前端必须把 client_result 带上来,usecase 里再做必填校验
func (s *AttemptService) CheckAttempt(ctx context.Context, req *pb.CheckAttemptRequest) (*pb.CheckAttemptReply, error) {
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	res, err := s.uc.Check(ctx, id, toBizClientResult(req.GetClientResult()))
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

// toBizClientResult proto ClientCheckResult → biz 层类型
// nil 输入直接返回 nil,让 usecase 按"缺字段"逻辑处理(sandbox 会忽略,其它模式会 400)
func toBizClientResult(p *pb.ClientCheckResult) *attempt.ClientCheckResult {
	if p == nil {
		return nil
	}
	return &attempt.ClientCheckResult{
		Passed:   p.GetPassed(),
		ExitCode: int(p.GetExitCode()),
		Stdout:   p.GetStdout(),
		Stderr:   p.GetStderr(),
	}
}

// 注:曾实现 ExtendAttempt(通关后复盘续期)—— V1 改为纯前端客户端倒计时,
// 到期再调 TerminateAttempt,依赖后端 PassedGrace 自然清理,避免新 RPC 带来的
// proto regen 依赖。biz 层保留 Extend 方法供将来接回。

// HeartbeatAttemptHandler 轻量级 HTTP 心跳端点
//
// 为什么不走 proto:proto 加新 RPC 要跑 regen,心跳又是纯副作用("刷 LastActiveAt"),
// 没有复杂的请求 / 响应字段。直接用 srv.HandleFunc 注册,省掉 proto/grpc 开销。
//
// 契约:
//   - 路径   : POST /v1/attempts/{id}/heartbeat
//   - 请求体 : 空(忽略任何内容)
//   - 响应   : 200 + {"ok":true}      心跳成功
//              4xx + {"ok":false,"reason":"..."}    id 非法 / attempt 不存在 / attempt 已结束
//
// 前端契约:
//   - Scenario 页每 20s 调一次(只在文档可见 & 用户最近有交互时发)
//   - 4xx 响应前端都当"心跳失败",停止后续心跳,由 30s polling 的 NEEDS_RESTART
//     分支兜底处理"容器死掉"的 UX
func (s *AttemptService) HeartbeatAttemptHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")

	if r.Method != http.MethodPost {
		writeHeartbeatErr(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "heartbeat requires POST")
		return
	}

	// 路径形如 /v1/attempts/{id}/heartbeat,手工 split 避免引 mux 依赖
	// 这里不要求 {id} 做严格 route 匹配,TestHandler 层都走 regex 清洗
	const prefix = "/v1/attempts/"
	const suffix = "/heartbeat"
	p := r.URL.Path
	if !strings.HasPrefix(p, prefix) || !strings.HasSuffix(p, suffix) {
		writeHeartbeatErr(w, http.StatusBadRequest, "INVALID_PATH", "expected /v1/attempts/{id}/heartbeat")
		return
	}
	idStr := strings.TrimSuffix(strings.TrimPrefix(p, prefix), suffix)
	id, err := parseID(idStr)
	if err != nil {
		writeHeartbeatErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid attempt id")
		return
	}

	if err := s.uc.Heartbeat(r.Context(), id); err != nil {
		// kerrors 带 HTTP status,直接透出方便前端识别
		code := http.StatusInternalServerError
		reason := "UNKNOWN"
		msg := err.Error()
		if ke, ok := err.(*kerrors.Error); ok {
			code = int(ke.Code)
			if code < 100 || code >= 600 {
				code = http.StatusBadRequest
			}
			reason = ke.Reason
			msg = ke.Message
		}
		writeHeartbeatErr(w, code, reason, msg)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// writeHeartbeatErr 统一的错误响应写入,避免各分支自己 json.Marshal
func writeHeartbeatErr(w http.ResponseWriter, code int, reason, msg string) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      false,
		"reason":  reason,
		"message": msg,
	})
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
//
// ★ 2026-04-22 改造:ttyd 走后端同源反代(/v1/ttyd/{id}/),不再暴露宿主端口
//
// 原方案:直接返回 "http://localhost:{port}/",前端 iframe 指那里。
// 这在 COEP=credentialless 下会被 Chrome 当 cross-origin,CORP 不全时加载失败,
// 错误页文案"localhost 拒绝了我们的连接请求"极易误导排障方向。
//
// 新方案:返回 "/v1/ttyd/{attemptId}/" 相对路径:
//   - 浏览器解析成同源 URL(dev 走 vite proxy,prod 走反代),iframe 直接加载无 COEP 问题
//   - 后端 HTTP 层的 ttyd_proxy handler 按 attemptId 反查 HostPort,把 HTTP + WebSocket
//     都转发给 127.0.0.1:{port}
//   - 宿主端口不再外露,外部只能通过 API 经过后端鉴权才能摸到 ttyd
//
// TerminalURLTemplate 配置保留:兼容部署场景里手动把 ttyd 放外部 CDN / 直连端口的玩法。
// 若 tmpl 显式配置了非默认值 → 走 tmpl 渲染(老行为);
// 否则(tmpl 为空 / 默认) → 走同源 /v1/ttyd/{id}/。
func (s *AttemptService) terminalURL(a *model.OpslabsAttempt) string {
	if a.HostPort <= 0 {
		return ""
	}
	tmpl := s.opts.TerminalURLTemplate
	// 默认 / 空 → 走同源反代路径
	if tmpl == "" || tmpl == "http://{host}:{port}/" {
		return TtydProxyURLPrefix + strconv.FormatInt(a.ID, 10) + "/"
	}
	// 显式模板 → 保留老行为(方便未来接外部反代)
	host := s.opts.TerminalHost
	if host == "" {
		host = "localhost"
	}
	url := strings.ReplaceAll(tmpl, "{host}", host)
	url = strings.ReplaceAll(url, "{port}", strconv.Itoa(a.HostPort))
	url = strings.ReplaceAll(url, "{id}", strconv.FormatInt(a.ID, 10))
	return url
}

// expiresAt LastActiveAt + 生效的空闲超时
// (场景级 > service 全局 > 30min 兜底)
func (s *AttemptService) expiresAt(a *model.OpslabsAttempt) time.Time {
	return a.LastActiveAt.Add(s.idleTimeoutFor(a.ScenarioSlug))
}

// idleTimeoutFor 按场景取 idle_timeout:场景覆盖优先,否则 service 全局默认
// 未注册场景走 service 默认(30min 兜底)
func (s *AttemptService) idleTimeoutFor(slug string) time.Duration {
	if sc, err := s.registry.Get(slug); err == nil {
		if sc.Runtime.IdleTimeoutMinutes > 0 {
			return time.Duration(sc.Runtime.IdleTimeoutMinutes) * time.Minute
		}
	}
	if s.opts.DefaultIdleTimeout > 0 {
		return s.opts.DefaultIdleTimeout
	}
	return 30 * time.Minute
}

// passedGraceFor 场景通关后宽限时间:场景覆盖 > service 默认 > 10min 兜底
func (s *AttemptService) passedGraceFor(slug string) time.Duration {
	if sc, err := s.registry.Get(slug); err == nil {
		if sc.Runtime.PassedGraceMinutes > 0 {
			return time.Duration(sc.Runtime.PassedGraceMinutes) * time.Minute
		}
	}
	if s.opts.DefaultPassedGrace > 0 {
		return s.opts.DefaultPassedGrace
	}
	return 10 * time.Minute
}

// runtimeEntry 按 slug 从 registry 反查 execution_mode + bundle_url
//
// 返回值:
//   - mode      : 永远有值(注册缺失时退化为 ExecutionModeSandbox,兜底不让前端拿空串)
//   - bundleURL : 仅 static / wasm-linux / web-container 返回非空
//
// registry 找不到场景时静默退化 —— 这种情况只可能出现在场景下架后还有存量 Attempt,
// 让前端拿到 sandbox 模式空 bundle_url,配合 TerminalURL 空判断降级到占位提示。
func (s *AttemptService) runtimeEntry(slug string) (mode, bundleURL string) {
	sc, err := s.registry.Get(slug)
	if err != nil {
		return scenario.DefaultExecutionMode, ""
	}
	mode = sc.EffectiveExecutionMode()
	bundleURL = bundleURLForMode(mode, sc.Slug)
	return mode, bundleURL
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
