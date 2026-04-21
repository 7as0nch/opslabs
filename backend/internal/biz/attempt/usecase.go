/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: AttemptUsecase:场景尝试的生命周期编排
 *               Start → (Get / Check / Terminate) → 结束(落库 + 释放容器)
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

	now := time.Now()
	a := &Attempt{
		ScenarioSlug: slug,
		Status:       StatusRunning,
		StartedAt:    now,
		LastActiveAt: now,
		CheckCount:   0,
	}
	a.New() // 生成雪花 ID
	// 尚未接入登录,默认匿名
	a.UserID = 0

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
			return nil, kerrors.New(503, "PORT_POOL_EXHAUSTED", "no free port")
		}
		uc.log.Error("runner run failed", zap.Error(err), zap.String("slug", slug), zap.Int64("id", a.ID))
		return nil, kerrors.InternalServer("CONTAINER_START_FAIL", "container start failed: "+err.Error())
	}

	a.ContainerID = res.ContainerID
	a.HostPort = res.HostPort

	if err := uc.repo.Create(ctx, a); err != nil {
		// 持久化失败:停掉容器避免泄露
		if stopErr := uc.runner.Stop(context.Background(), res.ContainerID); stopErr != nil {
			uc.log.Error("rollback stop failed",
				zap.Error(stopErr),
				zap.String("container_id", res.ContainerID))
		}
		uc.log.Error("repo create failed", zap.Error(err), zap.Int64("id", a.ID))
		return nil, kerrors.InternalServer("UNKNOWN", "persist attempt failed")
	}

	uc.store.Put(a)
	return a, nil
}

// ==============================================================
// Get
// ==============================================================

// Get 查询 attempt:先内存,再 DB
// 不会刷新 LastActiveAt,纯读;心跳应走 Heartbeat/Check
func (uc *AttemptUsecase) Get(ctx context.Context, id int64) (*Attempt, error) {
	if id <= 0 {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "invalid attempt id")
	}
	if a, ok := uc.store.Get(id); ok {
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
	uc.store.UpdateLastActive(id, time.Now())
	return nil
}

// ==============================================================
// Check
// ==============================================================

// Check 在容器里跑 check.sh,解析结果并落库
func (uc *AttemptUsecase) Check(ctx context.Context, id int64) (*CheckResult, error) {
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

	timeout := time.Duration(sc.Grading.CheckTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	execRes, err := uc.runner.Exec(ctx, a.ContainerID, sc.Grading.CheckScript, timeout)
	if err != nil {
		uc.log.Error("runner exec failed",
			zap.Error(err),
			zap.Int64("id", id),
			zap.String("container_id", a.ContainerID))
		return nil, kerrors.InternalServer("CHECK_EXEC_FAIL", "exec failed: "+err.Error())
	}

	successMark := sc.Grading.SuccessOutput
	if successMark == "" {
		successMark = "OK"
	}
	passed := execRes.ExitCode == 0 && strings.Contains(execRes.Stdout, successMark)

	// 无论通过与否都 +1,方便统计
	a.CheckCount++
	now := time.Now()
	a.LastActiveAt = now
	if passed {
		a.MarkPassed(now)
	}

	if err := uc.repo.Update(ctx, a); err != nil {
		uc.log.Error("update attempt after check failed", zap.Error(err), zap.Int64("id", id))
		// 不阻塞用户,返回 check 结果;下次 Check/GC 会再次尝试写入
	}
	uc.store.UpdateStatus(a)

	// 通关后异步停容器(先不阻塞接口返回,passed grace 期内还能让用户复盘)
	// 这里保守起见:running 状态还在,仅等 GC 或用户手动 Terminate
	return &CheckResult{
		Passed:   passed,
		ExitCode: execRes.ExitCode,
		Stdout:   execRes.Stdout,
		Stderr:   execRes.Stderr,
		Attempt:  a,
	}, nil
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
	uc.store.UpdateStatus(a)
	// 可选:从 store 里删掉,节省内存;这里保留让 Get 还能读到终态
	// uc.store.Delete(id)
	return nil
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
	// DB 保持 passed 状态,只是从内存缓存移除
	uc.store.Delete(id)
	return nil
}

// RestoreRunning 启动时把 DB 中 running 的 attempt 回灌到内存缓存
// 遇到异常不阻塞启动,只记日志 —— 容器真实死了的会在下一次 reaper 循环被标记 terminated
func (uc *AttemptUsecase) RestoreRunning(ctx context.Context) error {
	list, err := uc.repo.ListRunning(ctx)
	if err != nil {
		return err
	}
	for _, a := range list {
		uc.store.Put(a)
	}
	uc.log.Info("restored running attempts from db", zap.Int("count", len(list)))
	return nil
}
