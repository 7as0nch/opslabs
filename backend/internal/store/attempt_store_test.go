/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: AttemptStore 并发安全 + 拷贝语义基础断言
**/
package store

import (
	"sync"
	"testing"
	"time"

	"github.com/7as0nch/backend/models/generator/model"
)

// newTestAttempt 构造一条测试用 Attempt
func newTestAttempt(id int64, slug string) *model.OpslabsAttempt {
	a := &model.OpslabsAttempt{
		ScenarioSlug: slug,
		Status:       model.AttemptStatusRunning,
		StartedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}
	a.ID = id
	return a
}

func TestAttemptStore_PutGet(t *testing.T) {
	s := NewAttemptStore()
	a := newTestAttempt(1, "hello-world")
	s.Put(a)

	got, ok := s.Get(1)
	if !ok {
		t.Fatal("expected to find attempt 1")
	}
	if got.ScenarioSlug != "hello-world" {
		t.Fatalf("slug mismatch: %s", got.ScenarioSlug)
	}

	// 修改返回值不应影响缓存
	got.ScenarioSlug = "mutated"
	got2, _ := s.Get(1)
	if got2.ScenarioSlug != "hello-world" {
		t.Fatalf("cache got mutated: %s", got2.ScenarioSlug)
	}
}

func TestAttemptStore_DeleteIdempotent(t *testing.T) {
	s := NewAttemptStore()
	s.Put(newTestAttempt(2, "x"))
	s.Delete(2)
	s.Delete(2)
	if _, ok := s.Get(2); ok {
		t.Fatal("expected attempt 2 to be gone")
	}
}

func TestAttemptStore_UpdateLastActive(t *testing.T) {
	s := NewAttemptStore()
	s.Put(newTestAttempt(3, "x"))

	future := time.Now().Add(5 * time.Minute)
	if !s.UpdateLastActive(3, future) {
		t.Fatal("expected update to succeed")
	}
	got, _ := s.Get(3)
	if !got.LastActiveAt.Equal(future) {
		t.Fatalf("last_active not updated")
	}

	// 不存在的 id 应返回 false
	if s.UpdateLastActive(999, future) {
		t.Fatal("expected false for missing id")
	}
}

func TestAttemptStore_Snapshot(t *testing.T) {
	s := NewAttemptStore()
	for i := int64(1); i <= 5; i++ {
		s.Put(newTestAttempt(i, "x"))
	}
	snap := s.Snapshot()
	if len(snap) != 5 {
		t.Fatalf("expected 5 items, got %d", len(snap))
	}
	// 改快照不应该影响内部
	snap[0].ScenarioSlug = "mutated"
	for _, a := range s.Snapshot() {
		if a.ScenarioSlug == "mutated" {
			t.Fatal("snapshot mutation leaked into store")
		}
	}
}

func TestAttemptStore_Concurrent(t *testing.T) {
	s := NewAttemptStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := int64(i)
			s.Put(newTestAttempt(id, "x"))
			s.UpdateLastActive(id, time.Now())
			_, _ = s.Get(id)
			_ = s.Snapshot()
		}(i)
	}
	wg.Wait()
	if s.Len() != 50 {
		t.Fatalf("expected 50 items, got %d", s.Len())
	}
}
