/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 端口池,预分配 start-end 的 TCP 端口供容器 ttyd 映射
**/
package runtime

import "sync"

// PortPool 简易端口池,并发安全
//
// 三种端口状态:
//   - free : 空闲,等待 Acquire 分发
//   - used : 已被某次 attempt 占用,等待 Release 归还
//   - bad  : 经实测 OS 层已被其它进程绑定(常见:上次崩溃残留的 docker 容器)
//            一旦标 bad 就不再投放给 Acquire,直到进程重启或 Reset
type PortPool struct {
	mu   sync.Mutex
	free []int
	used map[int]bool
	bad  map[int]bool
}

// NewPortPool 构造端口池,[start, end] 闭区间都可用
func NewPortPool(start, end int) *PortPool {
	p := &PortPool{
		free: make([]int, 0, end-start+1),
		used: make(map[int]bool, end-start+1),
		bad:  make(map[int]bool),
	}
	for port := start; port <= end; port++ {
		p.free = append(p.free, port)
	}
	return p
}

// Acquire 取一个空闲端口,没有则返回 ErrPortPoolExhausted
func (p *PortPool) Acquire() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.free) == 0 {
		return 0, ErrPortPoolExhausted
	}
	n := len(p.free) - 1
	port := p.free[n]
	p.free = p.free[:n]
	p.used[port] = true
	return port, nil
}

// Release 归还端口。重复 release 或 release 未分配端口都是 no-op
//
// 注:bad 端口不会因为 Release 重新进入 free —— 调用方调 Release 表示这次
// attempt 不再用该端口,但端口本身是否真的可用仍需另行判定。
func (p *PortPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.used[port] {
		return
	}
	delete(p.used, port)
	if p.bad[port] {
		// 已知坏端口不回流到 free,等待 Reset / 进程重启
		return
	}
	p.free = append(p.free, port)
}

// MarkBad 把一个端口标记为不可用(Acquire 不再发放,Release 不再回流)
//
// 触发时机:DockerRunner.Run 拿到 "port is already allocated" 错误时调,
// 把该端口从池子里隔离掉,避免下一次 Acquire 立刻又拿到同一个端口造成死循环。
//
// 重复 MarkBad 是幂等的;MarkBad 一个不在 [start,end] 区间的端口也是 no-op。
func (p *PortPool) MarkBad(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bad[port] {
		return
	}
	p.bad[port] = true
	// 同步从 used 摘掉(如果当前正持有)
	delete(p.used, port)
	// 如果还在 free 里(罕见路径:外部并发 Release + MarkBad),也摘出来
	for i, v := range p.free {
		if v == port {
			p.free = append(p.free[:i], p.free[i+1:]...)
			break
		}
	}
}

// InUse 返回当前已占用端口数,用于监控
func (p *PortPool) InUse() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.used)
}

// BadCount 当前已隔离的坏端口数,用于监控/告警
func (p *PortPool) BadCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.bad)
}

// Capacity 返回端口池总容量(不含 bad)
func (p *PortPool) Capacity() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.free) + len(p.used)
}
