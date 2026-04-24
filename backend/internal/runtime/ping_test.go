/* *
 * @Author: chengjiang
 * @Date: 2026-04-22
 * @Description: Runner.Ping 单测
 *               DockerRunner 走 docker CLI 不便于纯单测(CI 上没 docker daemon),
 *               这里用一个假的 docker 二进制脚本 + MockRunner 组合覆盖关键路径:
 *                 - MockRunner 场景:Run 后 containerID 在 attempts 里 → nil
 *                 - MockRunner 场景:Stop 后 containerID 被清除 → ErrContainerNotFound
 *                 - MockRunner 场景:空 containerID → ErrContainerNotFound
 *                 - DockerRunner 场景:走一个返回 "true" 的假 docker 脚本 → nil
 *                 - DockerRunner 场景:走一个返回 "false" 的假 docker 脚本 → 非 nil
 *                 - DockerRunner 场景:走一个模拟 "No such object" 的假脚本 → ErrContainerNotFound
 *
 * 假脚本仅在 linux/mac 下有意义(用 /bin/sh 运行 shebang);Windows 开发机请在 WSL
 * 或 git-bash 下跑 `go test ./internal/runtime/...`,与项目其它 test 一致。
**/
package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/zap"
)

// ---- MockRunner ----

func TestMockRunner_Ping_After_Run_Returns_Nil(t *testing.T) {
	r := NewMockRunner()
	res, err := r.Run(context.Background(), RunSpec{
		Image:     "opslabs/dummy:v1",
		AttemptID: "attempt-1",
	})
	if err != nil {
		t.Fatalf("mock run failed: %v", err)
	}
	if err := r.Ping(context.Background(), res.ContainerID); err != nil {
		t.Fatalf("expected Ping nil after Run, got %v", err)
	}
}

func TestMockRunner_Ping_After_Stop_Returns_NotFound(t *testing.T) {
	r := NewMockRunner()
	res, _ := r.Run(context.Background(), RunSpec{Image: "opslabs/dummy:v1"})
	if err := r.Stop(context.Background(), res.ContainerID); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	err := r.Ping(context.Background(), res.ContainerID)
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound after Stop, got %v", err)
	}
}

func TestMockRunner_Ping_Empty_ContainerID(t *testing.T) {
	r := NewMockRunner()
	if err := r.Ping(context.Background(), ""); !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound for empty id, got %v", err)
	}
}

// ---- DockerRunner via fake docker binary ----
//
// writeFakeDocker 把一个可执行 shell 脚本写进临时目录,供 DockerRunner 作为 `docker`
// 二进制路径。脚本的第一参数必为 `inspect`(Ping 调它),其余参数都被忽略。
//
// behaviors:
//   - "running": stdout 打印 "true\n" + exit 0
//   - "stopped": stdout 打印 "false\n" + exit 0
//   - "nosuch":  stderr 打印 "Error: No such object: xxx" + exit 1
//
// 在 Windows 上用不了 /bin/sh,测试整体 skip(t.Skip)
func writeFakeDocker(t *testing.T, behavior string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake docker shell script only works on unix-like OS")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	var body string
	switch behavior {
	case "running":
		body = "#!/bin/sh\necho true\n"
	case "stopped":
		body = "#!/bin/sh\necho false\n"
	case "nosuch":
		body = "#!/bin/sh\necho 'Error: No such object: zzz' 1>&2\nexit 1\n"
	default:
		t.Fatalf("unknown behavior %s", behavior)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	return path
}

func TestDockerRunner_Ping_Running(t *testing.T) {
	bin := writeFakeDocker(t, "running")
	r := NewDockerRunner(20000, 20010, zap.NewNop(), WithDockerBin(bin))
	if err := r.Ping(context.Background(), "cid-running"); err != nil {
		t.Fatalf("expected nil when fake docker says running, got %v", err)
	}
}

func TestDockerRunner_Ping_Stopped_Returns_Error(t *testing.T) {
	bin := writeFakeDocker(t, "stopped")
	r := NewDockerRunner(20000, 20010, zap.NewNop(), WithDockerBin(bin))
	err := r.Ping(context.Background(), "cid-stopped")
	if err == nil {
		t.Fatalf("expected non-nil err when container stopped, got nil")
	}
	// 应该不是 ErrContainerNotFound(那是"容器不存在"的专属信号)
	if errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected wrapping err, not ErrContainerNotFound, got %v", err)
	}
}

func TestDockerRunner_Ping_NoSuch_Returns_NotFound(t *testing.T) {
	bin := writeFakeDocker(t, "nosuch")
	r := NewDockerRunner(20000, 20010, zap.NewNop(), WithDockerBin(bin))
	err := r.Ping(context.Background(), "cid-missing")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound for 'No such object', got %v", err)
	}
}

func TestDockerRunner_Ping_Empty(t *testing.T) {
	r := NewDockerRunner(20000, 20010, zap.NewNop())
	if err := r.Ping(context.Background(), ""); !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound for empty id, got %v", err)
	}
}
