/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: AttemptUsecase:场景尝试的生命周期编排
 *               Start → (Get / Check / Terminate) → 结束(落库 + 释放容器)
 *
 * ==============================================================
 * 容器销毁路径白名单(2026-04-22 确认,V1 只允许这 5 条,别的路径新增请先讨论)
 * ==============================================================
 *   1. 用户主动结束:service.TerminateAttempt → uc.Terminate
 *      触发:"放弃"按钮、已通关后用户关闭复盘面板、前端显式要重新开始
 *   2. idle 超时回收:server.GCServer.tick(running 分支)→ uc.Terminate
 *      触发:30min 没心跳 / Check,LastActiveAt 过期
 *   3. passed 复盘宽限到期:server.GCServer.tick(passed 分支)→ uc.CleanupPassed
 *      只停容器,status 保持 passed,保留通关记录供将来查
 *   4. Start 时 runner.Run 成功但 DB 持久化失败:usecase.startSandbox 回滚 runner.Stop
 *      避免僵尸容器(DB 没记录但容器还在占端口)
 *   5. stale-container 自愈:server.GCServer.cleanupStale / uc.GetWithPing
 *      容器被外部 docker rm / Docker Desktop 重启杀掉后,清 store + MarkTerminated,
 *      不再调 runner.Stop(容器已经不存在)
 *
 * ❌ 不应触发销毁的路径(曾经错过的,请不要加回来):
 *   - 前端 Scenario 页 useEffect 卸载("返回"按钮) ← P3 refused 元凶
 *   - 首页刷新 / tab 切换
 *   - Get 调用(只读,不改状态)
 *   - Check 失败(让用户继续调试,不打断)
 *
 * 修改这里任意一条销毁路径前,请先同步更新本注释。
 * ==============================================================
**/
package attempt

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/7as0nch/backend/internal/runtime"
	"github.com/7as0nch/backend/internal/scenario"
	"github.com/7as0nch/backend/internal/store"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"go.uber.org/zap"
)

// CheckResult 一次判题的结果快照,service 层翻译成 proto reply
type CheckResult struct {
	Passed   bool
	ExitCode int
	Stdout   string
	Stderr   string
	// Attempt 在本次 Check 后的快照(状态可能从 running -> passed)
	Attempt *Attempt
}

// ClientCheckResult 非 sandbox 执行模式下,前端 Runner 自跑判题后上报的结果
// service 层从 proto 翻译过来,usecase 不直接 import pb,保持分层干净
//   - Passed   : 前端判定是否通关
//   - ExitCode : 沿用 Unix 约定,0 成功,非 0 失败(2 代表硬错误/环境问题)
//   - Stdout / Stderr : 可选,用于日志 / 回显给用户
type ClientCheckResult struct {
	Passed   bool
	ExitCode int
	Stdout   string
	Stderr   string
}

// AttemptUsecase 场景尝试编排
type AttemptUsecase struct {
	repo     AttemptRepo
	store    *store.AttemptStore
	runner   runtime.Runner
	registry scenario.Registry
	log      *zap.Logger
}

// NewAttemptUsecase 构造 Usecase
func NewAttemptUsecase(
	repo AttemptRepo,
	st *store.AttemptStore,
	runner runtime.Runner,
	registry scenario.Registry,
	log *zap.Logger,
) *AttemptUsecase {
	return &AttemptUsecase{
		repo:     repo,
		store:    st,
		runner:   runner,
		registry: registry,
		log:      log,
	}
}

// ==============================================================
// Start
// ==============================================================

// Start 启动一次新场景尝试:分配容器 + 持久化 + 进内存缓存
//
// 复用策略:
//   - 用户回到同一 slug,如果 store 里已经有一个还活着(running / passed)的 attempt,
//     直接返回那一个,不再新建容器 / 新写 DB。
//   - 对 sandbox 模式尤其重要:每次 Start 都调 runner.Run 会反复 docker run,
//     既慢又会把用户之前在容器里的工作推掉(新容器是干净 rootfs)。
//   - 前端 Zustand 的 reuse 守卫靠 localStorage,清过 cache / 换浏览器都会失效;
//     后端这层兜底保证同进程里同一用户同一 slug 只会起一个容器。
//   - 沙盒驱动(mock / docker)都统一走这条路径;runtime 层再分。
//
// 未登录用户:靠前端传来的 clientID(localStorage uuid)区分"谁在做",
// 不同浏览器 / 清过 localStorage / 换设备都会拿到不同 clientID,各自独立 attempt,
// 不会互相串。接入登录后切回以 UserID 为主,clientID 仍可并存作多设备标识。
func (uc *AttemptUsecase) Start(ctx context.Context, slug string) (*Attempt, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "slug is empty")
	}

	sc, err := uc.registry.Get(slug)
	if err != nil {
		if errors.Is(err, scenario.ErrScenarioNotFound) {
			return nil, kerrors.NotFound("SCENARIO_NOT_FOUND", "scenario not found: "+slug)
		}
		uc.log.Error("registry get failed", zap.Error(err), zap.String("slug", slug))
		return nil, kerrors.InternalServer("UNKNOWN", "registry get failed")
	}

	// V1 未接入登录:owner 键是 clientID(X-Client-ID header),middleware 兜底到 "anon"
	clientID := ClientIDFromContext(ctx)

	// ===== 复用判定:命中则直接返回 =====
	// 先于任何新建逻辑,避免 runner.Run / repo.Create 白跑
	//
	// Redis 错误处理原则:FindActive / Get / UpdateLastActive / Delete 出网络错时
	// 只记 Warn,视为"未命中/未刷新",让主流程继续走新建路径,保证用户可用性。
	existing, ok, ferr := uc.store.FindActiveByClientSlug(ctx, clientID, slug)
	if ferr != nil {
		uc.log.Warn("store FindActiveByClientSlug failed, fallback to new attempt",
			zap.Error(ferr), zap.String("client_id", clientID), zap.String("slug", slug))
	}
	if ok {
		mode := sc.EffectiveExecutionMode()
		reusable := true
		// sandbox 模式下做两层校验:
		//   1. HostPort > 0           —— mock runner 给 0,过滤掉;正常 docker 一定非零
		//   2. runner.Ping(container) —— 真实探活,防止 docker rm 后 store 还记着导致
		//      Terminal iframe 连 ttyd 连不上(dial tcp 127.0.0.1:xxxxx: refused)
		if mode == scenario.ExecutionModeSandbox {
			if existing.HostPort <= 0 {
				uc.log.Warn("sandbox existing attempt has no host port, will recreate",
					zap.Int64("id", existing.ID), zap.String("slug", slug))
				reusable = false
			} else if pingErr := uc.runner.Ping(ctx, existing.ContainerID); pingErr != nil {
				uc.log.Warn("sandbox reuse ping failed, will recreate",
					zap.Int64("id", existing.ID),
					zap.String("container_id", existing.ContainerID),
					zap.Int("host_port", existing.HostPort),
					zap.Error(pingErr))
				reusable = false
			}
		}
		if reusable {
			uc.log.Info("attempt reused",
				zap.Int64("id", existing.ID),
				zap.String("slug", slug),
				zap.String("status", string(existing.Status)),
				zap.String("mode", mode),
			)
			// 标记活跃,顺便把 idle 计时拨回 now,等于"用户刚回来"
			if _, err := uc.store.UpdateLastActive(ctx, existing.ID, time.Now()); err != nil {
				uc.log.Warn("store UpdateLastActive on reuse failed",
					zap.Error(err), zap.Int64("id", existing.ID))
			}
			// 返回最新快照(UpdateLastActive 已把 LastActiveAt 刷到 Redis)
			if fresh, ok2, err := uc.store.Get(ctx, existing.ID); err == nil && ok2 {
				return fresh, nil
			}
			return existing, nil
		}
		// 旧 attempt 不能复用 —— 清 store 并尽量把 DB 也标记 terminated,
		// 这样前端下一次 Get 不会看到"running 但连不上"的僵尸记录。
		// 失败只记日志:新建路径下会写入一条全新 attempt,老的残留不会干扰当前请求。
		if err := uc.store.Delete(ctx, existing.ID); err != nil {
			uc.log.Warn("store Delete stale attempt failed",
				zap.Error(err), zap.Int64("id", existing.ID))
		}
		existing.MarkTerminated(time.Now())
		deadCtx, deadCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := uc.repo.Update(deadCtx, existing); err != nil {
			uc.log.Warn("mark stale attempt terminated failed",
				zap.Error(err),
				zap.Int64("id", existing.ID),
				zap.String("slug", slug))
		}
		deadCancel()
	}

	// =================================================================
	// 执行模式分流 —— V1 四模式都已接入
	// sandbox:起 Docker 容器 + ttyd,判题走 docker exec check.sh
	// static / wasm-linux / web-container:不分配容器,只落 Attempt 记录,
	//   前端 Runner 从 /v1/scenarios/{slug}/bundle/... 拉资源自己跑,
	//   判题时前端上报结果走 Check() 的前端上报分支
	// =================================================================
	mode := sc.EffectiveExecutionMode()
	now := time.Now()
	a := &Attempt{
		ScenarioSlug: slug,
		ClientID:     clientID, // 落库 owner 标识,GC / Get 探活后续都靠这字段关联
		Status:       StatusRunning,
		StartedAt:    now,
		LastActiveAt: now,
		CheckCount:   0,
	}
	a.New() // 生成雪花 ID
	// UserID 保留默认 0(未登录);登录接入后 service 层从 JWT claim 取 UserID 覆盖

	switch mode {
	case scenario.ExecutionModeSandbox:
		if err := uc.startSandbox(ctx, a, slug, sc); err != nil {
			return nil, err
		}
	case scenario.ExecutionModeStatic,
		scenario.ExecutionModeWasmLinux,
		scenario.ExecutionModeWebContainer:
		if err := uc.startBundleless(ctx, a); err != nil {
			return nil, err
		}
	default:
		uc.log.Error("unknown execution mode",
			zap.String("slug", slug),
			zap.String("mode", mode))
		return nil, kerrors.InternalServer("INVALID_EXECUTION_MODE",
			"unknown execution mode: "+mode)
	}

	// Put 走 Redis,失败视为硬错误:如果不能写 store,其他请求就找不回这个
	// attempt(Get/Terminate/ttyd_proxy 全失效),必须回滚容器避免泄露,同时让
	// 用户拿到明确错误码而不是"半成品" attempt。
	if err := uc.store.Put(ctx, a); err != nil {
		uc.log.Error("store Put after start failed, rolling back container",
			zap.Error(err), zap.Int64("id", a.ID), zap.String("slug", slug))
		if a.ContainerID != "" {
			if stopErr := uc.runner.Stop(context.Background(), a.ContainerID); stopErr != nil {
				uc.log.Error("rollback stop after store.Put failure",
					zap.Error(stopErr), zap.String("container_id", a.ContainerID))
			}
		}
		a.MarkTerminated(time.Now())
		deadCtx, deadCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if uerr := uc.repo.Update(deadCtx, a); uerr != nil {
			uc.log.Warn("mark rolled-back attempt terminated failed",
				zap.Error(uerr), zap.Int64("id", a.ID))
		}
		deadCancel()
		return nil, kerrors.InternalServer("STORE_WRITE_FAIL", "attempt cache write failed: "+err.Error())
	}
	return a, nil
}

// startSandbox 传统 Docker + ttyd 路径
// 失败时负责回滚(runner.Stop),保证不泄露僵尸容器
func (uc *AttemptUsecase) startSandbox(ctx context.Context, a *Attempt, slug string, sc *scenario.Scenario) error {
	spec := runtime.RunSpec{
		Image:       sc.Runtime.Image,
		MemoryMB:    sc.Runtime.MemoryMB,
		CPUs:        sc.Runtime.CPUs,
		NetworkMode: sc.Runtime.NetworkMode,
		Env: map[string]string{
			"OPSLABS_SLUG":       slug,
			"OPSLABS_ATTEMPT_ID": strconv.FormatInt(a.ID, 10),
		},
		AttemptID: strconv.FormatInt(a.ID, 10),
		Security: runtime.SecuritySpec{
			CapAdd:         sc.Runtime.Security.CapAdd,
			ReadonlyRootFS: sc.Runtime.Security.ReadonlyRootFS,
			TmpfsSizeMB:    sc.Runtime.Security.TmpfsSizeMB,
		},
	}

	res, err := uc.runner.Run(ctx, spec)
	if err != nil {
		if errors.Is(err, runtime.ErrPortPoolExhausted) {
			return kerrors.New(503, "PORT_POOL_EXHAUSTED", "no free port")
		}
		uc.log.Error("runner run failed", zap.Error(err), zap.String("slug", slug), zap.Int64("id", a.ID))
		return kerrors.InternalServer("CONTAINER_START_FAIL", "container start failed: "+err.Error())
	}

	a.ContainerID = res.ContainerID
	a.HostPort = res.HostPort

	// DB 写入给一个短超时 —— 远程 PG 抖动时不让整个请求挂死,让前端能尽快收到错误
	// 超时后走 rollback 分支停容器,避免僵尸容器
	createCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := uc.repo.Create(createCtx, a); err != nil {
		// 持久化失败:停掉容器避免泄露(用背景 ctx,保证不被父 ctx 超时牵连)
		if stopErr := uc.runner.Stop(context.Background(), res.ContainerID); stopErr != nil {
			uc.log.Error("rollback stop failed",
				zap.Error(stopErr),
				zap.String("container_id", res.ContainerID))
		}
		uc.log.Error("repo create failed",
			zap.Error(err),
			zap.Int64("id", a.ID),
			zap.String("container_id", res.ContainerID))
		// 区分超时和其它错误,前端/运维能一眼定位
		if errors.Is(err, context.DeadlineExceeded) {
			return kerrors.New(504, "DB_WRITE_TIMEOUT", "persist attempt timed out (5s) — check db health")
		}
		return kerrors.InternalServer("PERSIST_FAIL", "persist attempt failed: "+err.Error())
	}
	return nil
}

// startBundleless static / wasm-linux / web-container 共用的轻启动路径
//
//   - 不调 runner,不占端口,ContainerID / HostPort 留空
//   - 唯一动作是持久化一条 Attempt 记录,前端拿 attemptId 后从 bundle_url 拉资源
//   - bundle_url / execution_mode 不入库(它们由 scenario.Slug 通过 registry 计算得到,
//     没必要双写;DB schema 也就不用跟着多两列)
func (uc *AttemptUsecase) startBundleless(ctx context.Context, a *Attempt) error {
	createCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := uc.repo.Create(createCtx, a); err != nil {
		uc.log.Error("repo create failed (bundleless)",
			zap.Error(err),
			zap.Int64("id", a.ID),
			zap.String("slug", a.ScenarioSlug))
		if errors.Is(err, context.DeadlineExceeded) {
			return kerrors.New(504, "DB_WRITE_TIMEOUT", "persist attempt timed out (5s) — check db health")
		}
		return kerrors.InternalServer("PERSIST_FAIL", "persist attempt failed: "+err.Error())
	}
	return nil
}

// ==============================================================
// Get
// ==============================================================

// Get 查询 attempt:先 Redis store,再 DB 兜底
// 不会刷新 LastActiveAt,纯读;心跳应走 Heartbeat/Check
//
// Redis 错误:记 Warn 后直接降级到 DB,保证读路径可用
func (uc *AttemptUsecase) Get(ctx context.Context, id int64) (*Attempt, error) {
	if id <= 0 {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "invalid attempt id")
	}
	if a, ok, err := uc.store.Get(ctx, id); err != nil {
		uc.log.Warn("store Get failed, falling back to DB", zap.Error(err), zap.Int64("id", id))
	} else if ok {
		return a, nil
	}
	a, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return nil, kerrors.NotFound("ATTEMPT_NOT_FOUND", "attempt not found")
		}
		uc.log.Error("repo find failed", zap.Error(err), zap.Int64("id", id))
		return nil, kerrors.InternalServer("UNKNOWN", "find attempt failed")
	}
	return a, nil
}

// GetWithPing Get + 对 sandbox running attempt 做 runner.Ping 探活
//
// 为什么独立于 Get:
//   - Get 保持纯读语义(不修改 store/DB,不触发副作用),给无副作用需求的 caller 用
//   - 第二次进入场景的路径(service.GetAttempt)需要在此时就判"容器是否还活着",
//     否则前端拿到 status=running 的 attempt 去 iframe ttyd 会 "dial tcp refused"
//
// 行为:
//   - 非 sandbox(HostPort<=0 或 ContainerID 为空):直接 alive=true 返回,不做 Ping
//   - runner==nil(测试/mock 路径):alive=true,跳过 Ping
//   - Ping 失败:自动清 store + MarkTerminated 落库(和 GCServer.cleanupStale 同路径),
//     返回 alive=false,让 service 把"attempt 已失效"转成业务错误码,引导前端重新 Start
//
// 返回:
//   - attempt : 不管 alive 如何都返回,方便 caller 读 status / slug 等元信息
//   - alive   : false 仅出现在 sandbox+running+Ping 失败分支
//   - err     : 仅 Get 失败时返回;Ping 失败不算 err(内部已自愈)
func (uc *AttemptUsecase) GetWithPing(ctx context.Context, id int64) (*Attempt, bool, error) {
	a, err := uc.Get(ctx, id)
	if err != nil {
		return nil, false, err
	}
	// 非 running / 非 sandbox / 无端口 → 没什么可 Ping 的,直接 alive
	if a.Status != StatusRunning || a.HostPort <= 0 || a.ContainerID == "" {
		return a, true, nil
	}
	if uc.runner == nil {
		return a, true, nil
	}
	if pingErr := uc.runner.Ping(ctx, a.ContainerID); pingErr != nil {
		uc.log.Info("get: stale container detected, self-healing",
			zap.Int64("id", id),
			zap.String("container_id", a.ContainerID),
			zap.Int("host_port", a.HostPort),
			zap.Error(pingErr))
		// 清 Redis:让下一次请求直接走新 Start 路径
		if delErr := uc.store.Delete(ctx, id); delErr != nil {
			uc.log.Warn("get: store Delete stale failed (will retry via GC)",
				zap.Error(delErr), zap.Int64("id", id))
		}
		// 标 DB terminated:防止 reaper 重复扫到同一条记录,也让 history 查询能看到终态
		a.MarkTerminated(time.Now())
		deadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if uerr := uc.repo.Update(deadCtx, a); uerr != nil {
			uc.log.Warn("get: mark stale terminated failed (will retry via GC)",
				zap.Int64("id", id),
				zap.Error(uerr))
		}
		return a, false, nil
	}
	return a, true, nil
}

// Heartbeat 刷新 attempt 的 LastActiveAt,用于闲置超时检测
// 只刷新内存,不立刻落库(GC/停机时再 flush);未找到或已结束返回对应错误
func (uc *AttemptUsecase) Heartbeat(ctx context.Context, id int64) error {
	a, err := uc.Get(ctx, id)
	if err != nil {
		return err
	}
	if !a.IsActive() {
		return kerrors.Conflict("ATTEMPT_NOT_RUNNING", "attempt already finished")
	}
	// Heartbeat 只刷 LastActiveAt,Redis 失败不影响业务(前端下一次请求会再刷)
	if _, err := uc.store.UpdateLastActive(ctx, id, time.Now()); err != nil {
		uc.log.Warn("heartbeat: store UpdateLastActive failed", zap.Error(err), zap.Int64("id", id))
	}
	return nil
}

// ==============================================================
// Check
// ==============================================================

// Check 执行一次判题并落库
//
// 按执行模式分流:
//   - sandbox             : 后端 docker exec check.sh,忽略 client
//   - static / wasm-linux / web-container :
//       必须由前端 Runner 自跑完判题后把 client 带上来;
//       client==nil 时返回 FailedPrecondition,提醒前端接入
//
// 无论分支,都会自增 CheckCount、刷新 LastActiveAt,并在通过时 MarkPassed。
// repo.Update 失败不阻塞返回(下次 Check/GC 会再次尝试落库)。
func (uc *AttemptUsecase) Check(ctx context.Context, id int64, client *ClientCheckResult) (*CheckResult, error) {
	a, err := uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !a.IsActive() {
		return nil, kerrors.Conflict("ATTEMPT_NOT_RUNNING", "attempt already finished")
	}

	sc, err := uc.registry.Get(a.ScenarioSlug)
	if err != nil {
		uc.log.Error("scenario missing during check",
			zap.Error(err),
			zap.String("slug", a.ScenarioSlug),
			zap.Int64("id", id))
		return nil, kerrors.InternalServer("SCENARIO_NOT_FOUND", "scenario missing: "+a.ScenarioSlug)
	}

	var result *CheckResult
	switch sc.EffectiveExecutionMode() {
	case scenario.ExecutionModeSandbox:
		result, err = uc.checkSandbox(ctx, a, sc)
	case scenario.ExecutionModeStatic, scenario.ExecutionModeWasmLinux, scenario.ExecutionModeWebContainer:
		result, err = uc.checkFromClient(a, client)
	default:
		return nil, kerrors.InternalServer("INVALID_EXECUTION_MODE",
			"unknown execution mode: "+sc.EffectiveExecutionMode())
	}
	if err != nil {
		return nil, err
	}

	// 无论通过与否都 +1,方便统计
	a.CheckCount++
	now := time.Now()
	a.LastActiveAt = now
	if result.Passed {
		a.MarkPassed(now)
	}

	if err := uc.repo.Update(ctx, a); err != nil {
		uc.log.Error("update attempt after check failed", zap.Error(err), zap.Int64("id", id))
		// 不阻塞用户,返回 check 结果;下次 Check/GC 会再次尝试写入
	}
	// 同步回 Redis;失败只告警,不影响本次 check 结果返回
	if _, err := uc.store.UpdateStatus(ctx, a); err != nil {
		uc.log.Warn("store UpdateStatus after check failed", zap.Error(err), zap.Int64("id", id))
	}

	result.Attempt = a
	return result, nil
}

// checkSandbox 容器路径:docker exec check.sh 并匹配 SuccessOutput
func (uc *AttemptUsecase) checkSandbox(ctx context.Context, a *Attempt, sc *scenario.Scenario) (*CheckResult, error) {
	timeout := time.Duration(sc.Grading.CheckTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	execRes, err := uc.runner.Exec(ctx, a.ContainerID, sc.Grading.CheckScript, timeout)
	if err != nil {
		uc.log.Error("runner exec failed",
			zap.Error(err),
			zap.Int64("id", a.ID),
			zap.String("container_id", a.ContainerID))
		return nil, kerrors.InternalServer("CHECK_EXEC_FAIL", "exec failed: "+err.Error())
	}
	successMark := sc.Grading.SuccessOutput
	if successMark == "" {
		successMark = "OK"
	}
	passed := execRes.ExitCode == 0 && strings.Contains(execRes.Stdout, successMark)
	return &CheckResult{
		Passed:   passed,
		ExitCode: execRes.ExitCode,
		Stdout:   execRes.Stdout,
		Stderr:   execRes.Stderr,
	}, nil
}

// checkFromClient static / wasm-linux / web-container 的前端自判路径
//
// 后端只校验格式 + 存库,不重算 —— 防止跨模式泄露判题逻辑。
// 真要防爆解应走 sandbox 模式;bundle 的 runCheck 本身 view-source 就能看见。
//
// client==nil 视为 400 —— 前端该老老实实把结果带上。
func (uc *AttemptUsecase) checkFromClient(a *Attempt, client *ClientCheckResult) (*CheckResult, error) {
	if client == nil {
		return nil, kerrors.BadRequest("CLIENT_RESULT_REQUIRED",
			"non-sandbox mode requires client_result in request")
	}
	return &CheckResult{
		Passed:   client.Passed,
		ExitCode: client.ExitCode,
		Stdout:   client.Stdout,
		Stderr:   client.Stderr,
	}, nil
}

// ==============================================================
// Extend(续期)
// ==============================================================

// Extend 延长 attempt 的空闲超时
//
// 做法:把 LastActiveAt 向未来推移 extend,并且把 extend 记到 repo.Update。
// 这样 reaper 的 idle 判断( now - LastActiveAt > idleCutoff ) 自动生效,
// service 层算 expires_at = LastActiveAt + idleTimeout 也会线性延后。
//
// 适用状态:
//   - running :正常续期(例如将来做"keepalive")
//   - passed  :通关后进入复盘,extend 等于复盘时长
//
// 已经 terminated / expired / failed 的 attempt 禁止续期,返回 Conflict。
//
// maxExtend <= 0 时不设上限,service 层负责 clamp(默认 30min)。
func (uc *AttemptUsecase) Extend(ctx context.Context, id int64, extend time.Duration, reason string) (*Attempt, error) {
	if extend <= 0 {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "extend_seconds must be positive")
	}
	a, err := uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	switch a.Status {
	case StatusRunning, StatusPassed:
		// OK
	default:
		return nil, kerrors.Conflict("ATTEMPT_NOT_EXTENDABLE",
			"attempt status "+string(a.Status)+" cannot be extended")
	}

	// LastActiveAt 推到 now + extend — 注意这里不是 LastActiveAt + extend,
	// 否则对已经很久没活的 attempt 续期会"找不回"到 now 附近,用户会看到
	// 很诡异的倒计时(刚点完续期,倒计时还是负的)
	newActive := time.Now().Add(extend)
	a.LastActiveAt = newActive
	if err := uc.repo.Update(ctx, a); err != nil {
		uc.log.Error("update attempt on extend failed",
			zap.Error(err), zap.Int64("id", id), zap.String("reason", reason))
		return nil, kerrors.InternalServer("UNKNOWN", "persist extend failed")
	}
	if _, err := uc.store.UpdateLastActive(ctx, id, newActive); err != nil {
		uc.log.Warn("extend: store UpdateLastActive failed", zap.Error(err), zap.Int64("id", id))
	}
	uc.log.Info("attempt extended",
		zap.Int64("id", id),
		zap.Duration("extend", extend),
		zap.String("reason", reason),
		zap.Time("new_last_active_at", newActive),
	)
	return a, nil
}

// ==============================================================
// Terminate
// ==============================================================

// Terminate 主动结束(用户点"放弃"或 passed 后 grace 到期)
// 幂等:已经结束的 attempt 再调也不报错,原样返回
func (uc *AttemptUsecase) Terminate(ctx context.Context, id int64) error {
	a, err := uc.Get(ctx, id)
	if err != nil {
		return err
	}
	if !a.IsActive() {
		// 已结束,幂等直接返回
		return nil
	}

	if a.ContainerID != "" {
		if stopErr := uc.runner.Stop(ctx, a.ContainerID); stopErr != nil {
			// 容器停失败不阻塞状态落库,交给后台 reaper 兜底
			uc.log.Error("runner stop failed",
				zap.Error(stopErr),
				zap.Int64("id", id),
				zap.String("container_id", a.ContainerID))
		}
	}

	a.MarkTerminated(time.Now())
	if err := uc.repo.Update(ctx, a); err != nil {
		uc.log.Error("update attempt on terminate failed", zap.Error(err), zap.Int64("id", id))
		return kerrors.InternalServer("UNKNOWN", "persist terminate failed")
	}
	// Terminate 终态直接从 Redis 里移除:active SET + owner 索引都清掉
	// Round 6 起 Redis 是共享缓存,留着终态记录会占空间,Get 走 DB 兜底也能查到
	if err := uc.store.Delete(ctx, id); err != nil {
		uc.log.Warn("terminate: store Delete failed (TTL will clean up)",
			zap.Error(err), zap.Int64("id", id))
	}
	return nil
}

// MarkTerminatedInDB stale-container 场景专用:容器已经被外部杀掉,Attempt 已在
// 内存里置成 terminated/FinishedAt,这里只负责把这份快照写回 DB。
//
// 使用方:GCServer.cleanupStale —— 它自己做了 store.Delete + a.MarkTerminated,
// 只差把落库这一步委托给 usecase,避免 server 层直接摸 repo 破坏分层。
//
// 行为:
//   - 直接调 repo.Update,失败把 error 原样返回,让 caller 决定是否重试
//   - 不做任何业务校验(状态 / ContainerID),caller 已经在别处判过
//   - 不刷新 store(caller 已经 Delete),也不调 runner.Stop(容器已死)
func (uc *AttemptUsecase) MarkTerminatedInDB(ctx context.Context, a *Attempt) error {
	if a == nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "attempt is nil")
	}
	return uc.repo.Update(ctx, a)
}

// CleanupPassed passed 宽限期到了,清容器 + 从 store 里删,但保留 DB 的 passed 记录
// Reaper 调用路径:passed → grace 到期 → CleanupPassed
func (uc *AttemptUsecase) CleanupPassed(ctx context.Context, id int64) error {
	a, err := uc.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != StatusPassed {
		// 非 passed 不处理,交给别的清理路径
		return nil
	}
	if a.ContainerID != "" {
		if stopErr := uc.runner.Stop(ctx, a.ContainerID); stopErr != nil {
			uc.log.Error("cleanup passed: runner stop failed",
				zap.Error(stopErr),
				zap.Int64("id", id),
				zap.String("container_id", a.ContainerID))
		}
	}
	// DB 保持 passed 状态,只是从 Redis 缓存移除
	if err := uc.store.Delete(ctx, id); err != nil {
		uc.log.Warn("cleanup passed: store Delete failed (TTL will clean up)",
			zap.Error(err), zap.Int64("id", id))
	}
	return nil
}

// 注:原 RestoreRunning(进程启动从 DB 回灌内存 store)已于 Round 6 移除。
// AttemptStore 迁 Redis 后,跨进程 / 跨重启状态由 Redis 持有,前端 polling 的
// NEEDS_RESTART + GCServer stale-container 扫描 + Ping 兜底已足够处理边缘情况。
