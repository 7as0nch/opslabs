/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 容器运行时抽象接口,mock 与 docker 实现均需满足
**/
package runtime

import (
	"context"
	"errors"
	"time"
)

// 运行时错误
var (
	ErrPortPoolExhausted = errors.New("port pool exhausted")
	ErrContainerNotFound = errors.New("container not found")
)

// RunSpec 启动容器所需的参数
type RunSpec struct {
	// 镜像名,例如 opslabs/hello-world:v1
	Image string
	// 资源限制
	MemoryMB int64
	CPUs     float64
	// 网络模式:none / isolated / internet-allowed
	//   - none:            --network=none(完全断网,hello-world 类引导场景)
	//   - isolated(默认): --network=opslabs-scenarios(容器之间可互通,不连外网)
	//   - internet-allowed: 不指定 --network,走宿主默认 bridge,可访问外网
	NetworkMode string
	// 注入的环境变量
	Env map[string]string
	// AttemptID,用于打标签、日志关联
	AttemptID string
	// Security 容器安全选项,缺省值是"最严"(--cap-drop=ALL、readonly rootfs)
	// 需要 NET_ADMIN / iptables / systemctl 等的 ops 场景,显式声明
	Security SecuritySpec
}

// SecuritySpec 容器安全加固选项
//
// 全零值语义(默认最严中相对宽松,保证绝大多数场景能跑):
//   - --cap-drop=ALL
//   - --security-opt=no-new-privileges:true
//   - 不强制 --read-only(因为多数场景要写 /etc / /var 之类)
//
// 场景按需 opt-in 的典型值:
//   - hello-world:                    ReadonlyRootFS=true(答题只需 touch /tmp/*)
//   - ops-nginx-upstream-fail:        CapAdd=[NET_BIND_SERVICE](要监听低端口)
//   - linux-101-basic-shell:          ReadonlyRootFS=true
type SecuritySpec struct {
	// CapAdd 要追加的 Linux capabilities,例如 []string{"NET_ADMIN", "NET_BIND_SERVICE"}
	CapAdd []string
	// ReadonlyRootFS opt-in 只读根文件系统 + tmpfs(/tmp、/home/player、/run)
	ReadonlyRootFS bool
	// TmpfsSizeMB 当 ReadonlyRootFS=true 时,/tmp 与 /home/player tmpfs 大小,默认 64
	TmpfsSizeMB int
}

// RunResult 启动容器后返回的信息
type RunResult struct {
	ContainerID string
	// 宿主机映射端口(ttyd 7681 的外露端口)
	HostPort int
}

// ExecResult 判题脚本的执行结果
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner 容器运行时接口
//
// 实现说明:
//   - Week 1 先用 mock 实现,跑通 API 链路
//   - Week 1 后半程替换为真实 Docker SDK 实现
type Runner interface {
	// Run 启动容器,返回 containerID + 宿主机端口
	Run(ctx context.Context, spec RunSpec) (*RunResult, error)
	// Exec 在容器内以 root 执行脚本,超时后返回 ExitCode=124
	Exec(ctx context.Context, containerID, script string, timeout time.Duration) (*ExecResult, error)
	// Stop 停止并删除容器
	Stop(ctx context.Context, containerID string) error
}
