/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 系统菜单数据访问层
**/
package data

import (
	"context"

	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/internal/db"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/models/generator/query"
	"go.uber.org/zap"
)

type sysMenuRepo struct {
	db    db.DataRepo
	log   *zap.Logger
	query *query.Query  // 存储预编译的查询实例，避免重复获取DB连接
}

func NewSysMenuRepo(db db.DataRepo, log *zap.Logger) base.SysMenuRepo {
	// 单例模式：复用查询实例，避免频繁获取数据库连接
	return &sysMenuRepo{
		db:    db,
		log:   log,
		query: query.Use(db.GetDB()),
	}
}

// GetAll implements base.SysMenuRepo
func (r *sysMenuRepo) GetAll(ctx context.Context) ([]*model.SysMenu, error) {
	menus, err := r.query.SysMenu.WithContext(ctx).Order(r.query.SysMenu.Sort.Asc()).Find()
	if err != nil {
		return nil, err
	}
	return menus, nil
}

// GetRouter implements base.SysMenuRepo
func (r *sysMenuRepo) GetRouter(ctx context.Context) ([]*model.SysMenu, error) {
	menus, err := r.query.SysMenu.WithContext(ctx).
		Where(r.query.SysMenu.Type.In(uint8(model.MenuTypeDir), uint8(model.MenuTypeMenu))).
		Order(r.query.SysMenu.Sort.Asc()).
		Find()
	if err != nil {
		return nil, err
	}
	return menus, nil
}

// Add implements base.SysMenuRepo
func (r *sysMenuRepo) Add(ctx context.Context, menu *model.SysMenu) error {
	err := r.query.SysMenu.WithContext(ctx).Create(menu)
	if err != nil {
		r.log.Error("Add menu failed", zap.Error(err))
		return err
	}
	return nil
}

// Update implements base.SysMenuRepo
func (r *sysMenuRepo) Update(ctx context.Context, menu *model.SysMenu) error {
	rowsAffected, err := r.query.SysMenu.WithContext(ctx).Where(r.query.SysMenu.ID.Eq(menu.ID)).Updates(menu)
	if err != nil {
		r.log.Error("Update menu failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// Delete implements base.SysMenuRepo (注意：接口定义是 Delete 而不是 Remove)
func (r *sysMenuRepo) Delete(ctx context.Context, id int64) error {
	rowsAffected, err := r.query.SysMenu.WithContext(ctx).Where(r.query.SysMenu.ID.Eq(id)).Delete()
	if err != nil {
		r.log.Error("Delete menu failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// Get implements base.SysMenuRepo
func (r *sysMenuRepo) Get(ctx context.Context, id int64) (*model.SysMenu, error) {
	menu, err := r.query.SysMenu.WithContext(ctx).Where(r.query.SysMenu.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return menu, nil
}