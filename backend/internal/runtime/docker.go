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

// 端口冲突时最多重试几次。每次拿到的"坏端口"会被 MarkBad 永久隔离,
// 因此重试次数不需要太大;即使整个池都坏(极端情况)也会在这个上限内失败。
const dockerRunPortRetryLimit = 8

// Run docker run -d 起容器
//
// **端口冲突重试**:
// 在多个 backend 进程共用一台主机 / 上次崩溃残留 docker 容器还占着端口的场景,
// PortPool.Acquire 拿到的端口可能在 OS 层已被占用,docker 会回:
//   Bind for 0.0.0.0:19999 failed: port is already allocated
// 这种情况下我们把该端口 MarkBad(从池里隔离掉),换下一个端口重试,最多
// dockerRunPortRetryLimit 次。每次都失败就把最后一次错误抛上去,让用户能看到
// 具体冲突端口便于排查(比如手动 docker ps 看哪个旧容器占着)。
func (r *DockerRunner) Run(ctx context.Context, spec RunSpec) (*RunResult, error) {
	if spec.Image == "" {
		return nil, errors.New("docker: empty image")
	}

	var lastErr error
	for attempt := 0; attempt < dockerRunPortRetryLimit; attempt++ {
		res, err, conflict := r.tryRunOnce(ctx, spec)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !conflict {
			// 非端口冲突错误:直接失败,不重试
			return nil, err
		}
		r.log.Warn("docker port conflict, retrying with another port",
			zap.Int("attempt", attempt+1),
			zap.Error(err))
	}
	return nil, fmt.Errorf("docker run: exhausted %d port retries, last err: %w",
		dockerRunPortRetryLimit, lastErr)
}

// tryRunOnce 起一次容器,返回 (结果, 错误, 是否端口冲突)
//
// conflict=true 时:
//   - 端口已被 MarkBad,不会回流到 free 池
//   - 调用方应换一个端口重试
// conflict=false 时:
//   - 端口已 Release 回 free
//   - 错误是终态(配置错 / image 不存在等),不要重试
func (r *DockerRunner) tryRunOnce(ctx context.Context, spec RunSpec) (*RunResult, error, bool) {
	port, err := r.pool.Acquire()
	if err != nil {
		return nil, err, false
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
	args = append(args, spec.Image)

	out, err := r.run(ctx, args...)
	if err != nil {
		// 端口冲突识别 —— docker 不同版本/语言文案略有差异,统一按子串匹配
		if isPortConflictErr(out) || isPortConflictErr(err.Error()) {
			r.pool.MarkBad(port)
			// 容器名有 --name,如果 docker 在 publish 之前已经创建了同名容器壳子,
			// 顺手清掉避免下次 --name 冲突。绝大多数情况下 publish 失败容器根本
			// 不会被创建,这里 best-effort 静默处理。
			_, _ = r.run(context.Background(), "rm", "-f", name)
			return nil, fmt.Errorf("docker run: port %d already allocated (%s)",
				port, strings.TrimSpace(out)), true
		}
		r.pool.Release(port)
		return nil, fmt.Errorf("docker run: %w (%s)", err, strings.TrimSpace(out)), false
	}
	cid := strings.TrimSpace(out)
	if cid == "" {
		r.pool.Release(port)
		return nil, errors.New("docker run: empty container id"), false
	}
	// 只取前 12 位和 docker ps 一致,便于阅读
	if len(cid) > 64 {
		cid = cid[:64]
	}

	r.mu.Lock()
	r.containers[cid] = port
	r.mu.Unlock()

	return &RunResult{ContainerID: cid, HostPort: port}, nil, false
}

// isPortConflictErr 把端口冲突文案归一化判断
//
// docker CE 中文 / 英文 / 不同 OS 报文略有差异,这里按几个高频子串识别:
//   - "port is already allocated"             // Linux / Mac
//   - "Bind for 0.0.0.0:xxxx failed"          // 部分 Win/Mac
//   - "address already in use"                // Linux 端口本已被宿主进程占着
//   - "已分配" / "端口已"                       // 中文环境(打安全 wrap)
func isPortConflictErr(s string) bool {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "port is already allocated"):
		return true
	case strings.Contains(low, "bind for 0.0.0.0") && strings.Contains(low, "failed"):
		return true
	case strings.Contains(low, "address already in use"):
		return true
	case strings.Contains(s, "端口") && strings.Contains(s, "已"):
		return true
	}
	return false
}

// Reconcile 启动时一次性清理 opslabs.* 标签的残余容器
//
// 触发时机:进程启动早期(在开放 http/grpc 之前)。
// 目的:上次进程崩溃 / Ctrl-C 而没有走正常 Stop 链路时,docker 里仍残留
// "opslabs-xxxxxx" 容器,占着端口导致下一次 Run 必然冲突。
//
// 实现:
//   docker ps -aq --filter label=opslabs.attempt_id  # 列所有
//   docker rm -f $cid ...                            # 强制删
//
// 注:目前 V1 没有"跨进程恢复 attempt 上下文"的能力(restoreRunning 只
// 回灌内存 store,无法把容器挂回 runner.containers map),所以这里直接
// 全部 nuke 是最干净的做法。等以后做多副本 / 长 attempt 时再细化。
//
// 失败不阻塞启动 —— 顶多下次 Run 撞回端口冲突,有 retry 兜底。
func (r *DockerRunner) Reconcile(ctx context.Context) error {
	// list
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := r.run(listCtx, "ps", "-aq", "--filter", "label=opslabs.attempt_id")
	if err != nil {
		// docker daemon 不在?让上层只记日志,不阻塞
		return fmt.Errorf("docker ps: %w", err)
	}
	cids := strings.Fields(strings.TrimSpace(out))
	if len(cids) == 0 {
		r.log.Info("docker reconcile: no leftover opslabs containers")
		return nil
	}
	r.log.Warn("docker reconcile: removing leftover opslabs containers",
		zap.Int("count", len(cids)))
	rmCtx, cancel2 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel2()
	args := append([]string{"rm", "-f"}, cids...)
	if _, err := r.run(rmCtx, args...); err != nil {
		return fmt.Errorf("docker rm leftovers: %w", err)
	}
	return nil
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

// Ping 通过 docker inspect 校验容器是否仍在运行
//
// 实现细节:
//   - 直接用 `docker inspect -f '{{.State.Running}}' <cid>`,既快又无副作用
//   - stdout 为 "true\n" 视为活,"false\n" 视为已 exit/paused/created
//   - docker 返回 "No such object" / "No such container" 一律归一到 ErrContainerNotFound
//   - daemon 本身挂掉 → 返回原始 err,调用方自行决定兜底(不吞)
//
// 超时给 3s —— inspect 走本地 docker socket,正常耗时 < 50ms,3s 足以覆盖最糟情况;
// 上层复用判定若等太久不如走新建路径,不值得卡 start 流程
func (r *DockerRunner) Ping(ctx context.Context, containerID string) error {
	if containerID == "" {
		return ErrContainerNotFound
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := r.run(pingCtx, "inspect", "-f", "{{.State.Running}}", containerID)
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "no such object") ||
			strings.Contains(low, "no such container") ||
			strings.Contains(low, "error: no such") {
			return ErrContainerNotFound
		}
		// daemon 不可达 / inspect 语法错 / ctx 超时 —— 统一让上层拿到原始错
		return fmt.Errorf("docker inspect: %w", err)
	}
	running := strings.TrimSpace(out)
	if running == "true" {
		return nil
	}
	// "false" / "" / 其它 —— 容器存在但不是 running,照样视为不可复用
	return fmt.Errorf("docker inspect: container %s not running (state=%q)",
		containerID, running)
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
