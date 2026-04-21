/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Mock 运行时,不启动真实容器,也不分配真实端口
 *               用途:
 *                 - 后端单元测试,不需要 Docker Desktop
 *                 - 场景注册表/contract 冒烟(curl 打通 start/check/terminate 流程)
 *               关键行为:HostPort=0,service 层据此返回空的 terminalUrl,
 *               前端会识别并显示"Mock 预览模式",不会去 iframe 一个不存在的端口。
 *               ⚠️ 线上运行务必用 docker driver,mock 不提供真实终端体验。
**/
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MockRunner 假容器运行时(不占端口、不拉容器)
//
// 行为约定:
//   - Run:    返回 uuid 风格 containerID + HostPort=0
//   - Exec:   默认返回 {Stdout: "OK\n", ExitCode: 0};
//             可通过 SetExecResult(containerID, result) 或 SetExecResultBySlug
//             (配合 spec.Env["OPSLABS_SLUG"]) 覆盖
//   - Stop:   释放内存记录,幂等
type MockRunner struct {
	mu sync.Mutex
	// containerID -> attemptID,仅作调试
	attempts map[string]string
	// containerID -> override exec result(精确匹配优先)
	execOverride map[string]*ExecResult
	// slug -> override exec result(通过 Env["OPSLABS_SLUG"] 关联)
	slugOverride map[string]*ExecResult
	// 模拟 exec 的耗时,默认 0;测试里可以调大看超时
	execLatency time.Duration
}

// NewMockRunner 构造 —— 不再吃端口区间参数,调用方保持与 docker 分支签名一致即可
func NewMockRunner() *MockRunner {
	return &MockRunner{
		attempts:     make(map[string]string),
		execOverride: make(map[string]*ExecResult),
		slugOverride: make(map[string]*ExecResult),
	}
}

// Run 返回假 containerID + HostPort=0(让 service 层知道"没有真实终端")
func (m *MockRunner) Run(ctx context.Context, spec RunSpec) (*RunResult, error) {
	cid := mockContainerID()

	m.mu.Lock()
	m.attempts[cid] = spec.AttemptID
	if slug, ok := spec.Env["OPSLABS_SLUG"]; ok && slug != "" {
		if r, hit := m.slugOverride[slug]; hit {
			m.execOverride[cid] = r
		}
	}
	m.mu.Unlock()

	return &RunResult{
		ContainerID: cid,
		HostPort:    0, // 固定 0 —— 前端会识别为 mock 模式
	}, nil
}

// Exec 默认 OK,有覆盖则走覆盖
func (m *MockRunner) Exec(ctx context.Context, containerID, script string, timeout time.Duration) (*ExecResult, error) {
	m.mu.Lock()
	_, exists := m.attempts[containerID]
	override := m.execOverride[containerID]
	latency := m.execLatency
	m.mu.Unlock()

	if !exists {
		return nil, ErrContainerNotFound
	}

	// 模拟耗时 + 超时
	if latency > 0 {
		select {
		case <-time.After(latency):
		case <-ctx.Done():
			return &ExecResult{ExitCode: 124, Stderr: "context canceled"}, nil
		}
		if timeout > 0 && latency > timeout {
			return &ExecResult{ExitCode: 124, Stderr: "mock exec timeout"}, nil
		}
	}

	if override != nil {
		cp := *override
		return &cp, nil
	}
	return &ExecResult{
		Stdout:   "OK\n",
		ExitCode: 0,
	}, nil
}

// Stop 释放内存记录,幂等
func (m *MockRunner) Stop(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attempts[containerID]; !ok {
		return nil
	}
	delete(m.attempts, containerID)
	delete(m.execOverride, containerID)
	return nil
}

// ============ 测试辅助 API ============

// SetExecResult 精确指定某个 containerID 下一次 Exec 的返回
func (m *MockRunner) SetExecResult(containerID string, r *ExecResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execOverride[containerID] = r
}

// SetExecResultBySlug 按 slug 指定下一次 Run 时注入的 exec 覆盖
// 依赖 spec.Env["OPSLABS_SLUG"] 传入 slug
func (m *MockRunner) SetExecResultBySlug(slug string, r *ExecResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slugOverride[slug] = r
}

// SetExecLatency 调整 mock 的 exec 耗时,用于测试超时逻辑
func (m *MockRunner) SetExecLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execLatency = d
}

// ActiveCount 当前活跃的假 attempt 数,用于测试
func (m *MockRunner) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.attempts)
}

// ============ 工具 ============

// mockContainerID 生成 32 字符 hex 作为假 containerID,风格贴近 docker
func mockContainerID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("mock-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
