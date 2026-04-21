/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Attempt 内存缓存,承载"当前进程里还活着的场景尝试"
 *               作用:
 *                 - Check/Terminate 时快速拿到 containerID/hostPort,不必每次查 DB
 *                 - GC goroutine 扫描超时 attempt,批量落库 + 通知运行时停容器
 *               约束:
 *                 - 进程重启会丢,靠 repo.ListRunning() 在启动时回放
 *                 - 不依赖 biz 层,只持有 model.OpslabsAttempt,避免循环依赖
**/
package store

import (
	"sync"
	"time"

	"github.com/7as0nch/backend/models/generator/model"
)

// AttemptStore 并发安全的 Attempt 快照缓存
//
// 语义:
//   - Put:    新增或覆盖,写入不做去重检查,调用方负责
//   - Get:    拷贝返回,调用方可以随便改,不污染缓存
//   - Delete: 幂等
//   - UpdateLastActive: 心跳刷新;key 不存在时是 no-op,返回 false
//   - Snapshot: 返回当前所有条目的深拷贝切片,供 GC 扫描
type AttemptStore struct {
	mu   sync.RWMutex
	data map[int64]*model.OpslabsAttempt
}

// NewAttemptStore 构造空缓存
func NewAttemptStore() *AttemptStore {
	return &AttemptStore{
		data: make(map[int64]*model.OpslabsAttempt),
	}
}

// Put 写入/覆盖。传入的是指针,但存的是深拷贝,避免外部后续 mutate 影响缓存。
func (s *AttemptStore) Put(a *model.OpslabsAttempt) {
	if a == nil {
		return
	}
	cp := cloneAttempt(a)
	s.mu.Lock()
	s.data[a.ID] = cp
	s.mu.Unlock()
}

// Get 命中返回深拷贝,未命中 (nil, false)
func (s *AttemptStore) Get(id int64) (*model.OpslabsAttempt, bool) {
	s.mu.RLock()
	a, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return cloneAttempt(a), true
}

// Delete 删除,幂等
func (s *AttemptStore) Delete(id int64) {
	s.mu.Lock()
	delete(s.data, id)
	s.mu.Unlock()
}

// UpdateLastActive 心跳刷新,存在返回 true
func (s *AttemptStore) UpdateLastActive(id int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data[id]
	if !ok {
		return false
	}
	a.LastActiveAt = now
	return true
}

// UpdateStatus 更新状态机字段(MarkPassed 等已经改过 Attempt 后,调用这个同步回缓存)
// 传入的 a 被深拷贝后覆盖 map 中的条目。不存在则 no-op 返回 false。
func (s *AttemptStore) UpdateStatus(a *model.OpslabsAttempt) bool {
	if a == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[a.ID]; !ok {
		return false
	}
	s.data[a.ID] = cloneAttempt(a)
	return true
}

// Snapshot 返回当前所有活跃条目的深拷贝,调用方可自由遍历/过滤
//
// GC 场景:
//
//	list := store.Snapshot()
//	for _, a := range list {
//	    if now.Sub(a.LastActiveAt) > idleTimeout { ...回收... }
//	}
func (s *AttemptStore) Snapshot() []*model.OpslabsAttempt {
	s.mu.RLock()
	out := make([]*model.OpslabsAttempt, 0, len(s.data))
	for _, a := range s.data {
		out = append(out, cloneAttempt(a))
	}
	s.mu.RUnlock()
	return out
}

// Len 当前缓存条目数,用于监控/指标
func (s *AttemptStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// cloneAttempt 值拷贝 + 可变指针字段的深拷贝
func cloneAttempt(a *model.OpslabsAttempt) *model.OpslabsAttempt {
	if a == nil {
		return nil
	}
	cp := *a
	if a.FinishedAt != nil {
		t := *a.FinishedAt
		cp.FinishedAt = &t
	}
	if a.DurationMS != nil {
		v := *a.DurationMS
		cp.DurationMS = &v
	}
	return &cp
}
