/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 场景注册表基础断言
**/
package scenario

import (
	"errors"
	"testing"
)

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	slugs := []string{
		"hello-world",
		"frontend-devserver-down",
		"backend-api-500",
		"ops-nginx-upstream-fail",
	}
	for _, slug := range slugs {
		s, err := r.Get(slug)
		if err != nil {
			t.Fatalf("get %s: %v", slug, err)
		}
		if s.Slug != slug {
			t.Fatalf("slug mismatch: want %s got %s", slug, s.Slug)
		}
		if s.Title == "" {
			t.Fatalf("%s: empty title", slug)
		}
		if s.Grading.CheckScript == "" {
			t.Fatalf("%s: empty check_script", slug)
		}
		if s.Runtime.Image == "" {
			t.Fatalf("%s: empty image", slug)
		}
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("definitely-not-a-real-slug")
	if !errors.Is(err, ErrScenarioNotFound) {
		t.Fatalf("expected ErrScenarioNotFound, got %v", err)
	}
}

func TestRegistry_ListOrder(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 4 {
		t.Fatalf("expected 4 scenarios, got %d", len(list))
	}
	// 按难度升序
	for i := 1; i < len(list); i++ {
		if list[i].Difficulty < list[i-1].Difficulty {
			t.Fatalf("list not sorted by difficulty: %+v", list)
		}
	}
	// hello-world 最简单,应排第一
	if list[0].Slug != "hello-world" {
		t.Fatalf("expected hello-world first, got %s", list[0].Slug)
	}
}
