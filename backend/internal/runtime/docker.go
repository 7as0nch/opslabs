/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Docker 运行时实现(Phase B)
 *               走 docker CLI (exec.Command) 方案,避免引入 docker SDK 的依赖链
 *               Windows Docker Desktop / Linux Docker Engine 均可透明支持
**/
package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DockerRunner 通过 docker CLI 管理场景容器
//
// 设计点:
//   - 每个 attempt 分配 [portStart, portEnd] 区间内的一个端口,映射到容器 ttyd 默认 7681
//   - --label opslabs.attempt_id=... 便于排查/批量清理
//   - --network 由配置注入,默认 opslabs-scenarios(没有会走 bridge)
//   - --cap-drop=ALL --security-opt no-new-privileges --pids-limit 防爆破
//   - Exec 走 docker exec -i <cid> bash -c,超时由 ctx 控制
type DockerRunner struct {
	pool    *PortPool
	log     *zap.Logger
	network string
	// ttyd 在容器内的端口,标准 7681
	containerTtyPort int
	// docker 二进制路径,默认 "docker"
	dockerBin string

	mu         sync.Mutex
	containers map[string]int // containerID -> hostPort
}

// DockerOption 构造选项
type DockerOption func(*DockerRunner)

// WithDockerNetwork 指定 docker network,默认 bridge
func WithDockerNetwork(n string) DockerOption {
	return func(r *DockerRunner) { r.network = n }
}

// WithDockerBin 覆盖 docker 二进制路径
func WithDockerBin(b string) DockerOption {
	return func(r *DockerRunner) { r.dockerBin = b }
}

// WithContainerTtyPort 覆盖容器内 ttyd 端口,默认 7681
func WithContainerTtyPort(p int) DockerOption {
	return func(r *DockerRunner) { r.containerTtyPort = p }
}

// NewDockerRunner 构造 DockerRunner
func NewDockerRunner(portStart, portEnd int, log *zap.Logger, opts ...DockerOption) *DockerRunner {
	r := &DockerRunner{
		pool:             NewPortPool(portStart, portEnd),
		log:              log,
		network:          "",
		containerTtyPort: 7681,
		dockerBin:        "docker",
		containers:       make(map[string]int),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run docker run -d 起容器
func (r *DockerRunner) Run(ctx context.Context, spec RunSpec) (*RunResult, error) {
	port, err := r.pool.Acquire()
	if err != nil {
		return nil, err
	}

	name := "opslabs-" + dockerRandHex(6)
	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "opslabs.attempt_id=" + spec.AttemptID,
		"--label", "opslabs.slug=" + spec.Env["OPSLABS_SLUG"],
		"--publish", fmt.Sprintf("%d:%d", port, r.containerTtyPort),
		// ====== 安全加固(默认最严) ======
		"--pids-limit", "256",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true",
		"--ulimit", "nofile=1024:2048",
	}
	// 按需加回 capabilities(例如 ops 场景要 NET_ADMIN / NET_BIND_SERVICE)
	for _, c := range spec.Security.CapAdd {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		args = append(args, "--cap-add", c)
	}
	// ReadonlyRootFS 是 opt-in:场景显式打开时启用 --read-only + 三条 tmpfs
	// (ops 等需要改配置的场景保持默认 writable)
	if spec.Security.ReadonlyRootFS {
		tmpfsMB := spec.Security.TmpfsSizeMB
		if tmpfsMB <= 0 {
			tmpfsMB = 64
		}
		args = append(args,
			"--read-only",
			"--tmpfs", fmt.Sprintf("/tmp:rw,size=%dm,mode=1777", tmpfsMB),
			"--tmpfs", fmt.Sprintf("/home/player:rw,uid=1000,gid=1000,size=%dm,mode=0755", tmpfsMB),
			"--tmpfs", "/run:rw,size=16m,mode=0755",
		)
	}
	if spec.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", spec.MemoryMB))
	}
	if spec.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.CPUs, 'f', 2, 64))
	}
	// NetworkMode:
	//   "none"             → 完全断网(hello-world 这种不需要任何联网)
	//   "internet-allowed" → 不指定,走默认 bridge,可连外网
	//   其他(含 isolated/空):如果 r.network 非空,接入 opslabs-scenarios 网络
	switch spec.NetworkMode {
	case "none":
		args = append(args, "--network", "none")
	case "internet-allowed":
		// no-op,走 docker 默认 bridge
	default:
		if r.network != "" {
			args = append(args, "--network", r.network)
		}
	}
	for k, v := range spec.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if spec.Image == "" {
		r.pool.Release(port)
		return nil, errors.New("docker: empty image")
	}
	args = append(args, spec.Image)

	out, err := r.run(ctx, args...)
	if err != nil {
		r.pool.Release(port)
		return nil, fmt.Errorf("docker run: %w (%s)", err, strings.TrimSpace(out))
	}
	cid := strings.TrimSpace(out)
	if cid == "" {
		r.pool.Release(port)
		return nil, errors.New("docker run: empty container id")
	}
	// 只取前 12 位和 docker ps 一致,便于阅读
	if len(cid) > 64 {
		cid = cid[:64]
	}

	r.mu.Lock()
	r.containers[cid] = port
	r.mu.Unlock()

	return &RunResult{ContainerID: cid, HostPort: port}, nil
}

// Exec docker exec -u root -i <cid> bash -c <script>
// 强制以 root 身份跑 —— check.sh 是 0700 owner=root,player 看不到也跑不了,
// 这是题目答案防泄漏的第一道闸;判题时 runtime 必须以 root 进去执行。
func (r *DockerRunner) Exec(ctx context.Context, containerID, script string, timeout time.Duration) (*ExecResult, error) {
	if containerID == "" {
		return nil, ErrContainerNotFound
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, r.dockerBin,
		"exec", "-u", "root", "-i", containerID, "bash", "-lc", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exit := 0
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return &ExecResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String() + "\n[opslabs] exec timeout",
				ExitCode: 124,
			}, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exit = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("docker exec: %w", err)
		}
	}
	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exit,
	}, nil
}

// Stop docker rm -f
func (r *DockerRunner) Stop(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	// 10s 兜底,不让 Stop 无限卡住
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := r.run(stopCtx, "rm", "-f", containerID)
	// 无论成功与否,都归还端口 + 清理 map(避免泄漏)
	r.mu.Lock()
	port, ok := r.containers[containerID]
	delete(r.containers, containerID)
	r.mu.Unlock()
	if ok {
		r.pool.Release(port)
	}
	if err != nil {
		// "No such container" 视为已清理,幂等返回
		if strings.Contains(err.Error(), "No such container") ||
			strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("docker rm: %w", err)
	}
	return nil
}

// InUsePorts 当前占用端口数,监控/测试用
func (r *DockerRunner) InUsePorts() int {
	return r.pool.InUse()
}

// ==============================================================
// 内部工具
// ==============================================================

// run 统一封装 docker CLI 执行,捕获 stdout+stderr
func (r *DockerRunner) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.dockerBin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%w: %s", err, out.String())
	}
	return out.String(), nil
}

// dockerRandHex 生成容器名后缀
func dockerRandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
