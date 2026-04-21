/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 端口池,预分配 start-end 的 TCP 端口供容器 ttyd 映射
**/
package runtime

import "sync"

// PortPool 简易端口池,并发安全
type PortPool struct {
	mu   sync.Mutex
	free []int
	used map[int]bool
}

// NewPortPool 构造端口池,[start, end] 闭区间都可用
func NewPortPool(start, end int) *PortPool {
	p := &PortPool{
		free: make([]int, 0, end-start+1),
		used: make(map[int]bool, end-start+1),
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
func (p *PortPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.used[port] {
		return
	}
	delete(p.used, port)
	p.free = append(p.free, port)
}

// InUse 返回当前已占用端口数,用于监控
func (p *PortPool) InUse() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.used)
}

// Capacity 返回端口池总容量
func (p *PortPool) Capacity() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.free) + len(p.used)
}
