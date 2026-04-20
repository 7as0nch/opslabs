/* *
 * @Author: chengjiang
 * @Date: 2025-11-16 20:13:08
 * @Description:
**/
package base

import (
	"context"

	"github.com/example/aichat/backend/models/generator/model"
)

type SysMenuRepo interface {
	GetAll(ctx context.Context) ([]*model.SysMenu, error)
	GetRouter(ctx context.Context) ([]*model.SysMenu, error)
	Add(ctx context.Context, menu *model.SysMenu) error
	Update(ctx context.Context, menu *model.SysMenu) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.SysMenu, error)
}

type SysMenuUseCase struct {
	menu SysMenuRepo
}

func NewSysMenuUseCase(menu SysMenuRepo) *SysMenuUseCase {
	return &SysMenuUseCase{
		menu: menu,
	}
}

// GetAll
func (s *SysMenuUseCase) GetAll(ctx context.Context) ([]*model.SysMenu, error) {
	return s.menu.GetAll(ctx)
}

// GetRouter
func (s *SysMenuUseCase) GetRouter(ctx context.Context) ([]*model.SysMenu, error) {
	return s.menu.GetRouter(ctx)
}

// Add
func (s *SysMenuUseCase) Add(ctx context.Context, menu *model.SysMenu) error {
	return s.menu.Add(ctx, menu)
}

// Update
func (s *SysMenuUseCase) Update(ctx context.Context, menu *model.SysMenu) error {
	return s.menu.Update(ctx, menu)
}

// Delete
func (s *SysMenuUseCase) Delete(ctx context.Context, id int64) error {
	return s.menu.Delete(ctx, id)
}

// Get
func (s *SysMenuUseCase) Get(ctx context.Context, id int64) (*model.SysMenu, error) {
	return s.menu.Get(ctx, id)
}
