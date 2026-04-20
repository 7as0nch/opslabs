/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 字典类型数据访问层
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

type dictTypeRepo struct {
	db  db.DataRepo
	log *zap.Logger
}

func NewDictTypeRepo(db db.DataRepo, log *zap.Logger) base.DictTypeRepo {
	return &dictTypeRepo{
		db:  db,
		log: log,
	}
}

// GetAll implements base.DictTypeRepo
func (r *dictTypeRepo) GetAll(ctx context.Context) ([]*model.SysDictType, error) {
	dictTypes, err := query.Use(r.db.DB(ctx)).SysDictType.WithContext(ctx).Find()
	if err != nil {
		return nil, err
	}
	return dictTypes, nil
}

// GetByTypeCode implements base.DictTypeRepo
func (r *dictTypeRepo) GetByTypeCode(ctx context.Context, typeCode string) (*model.SysDictType, error) {
	q := query.Use(r.db.DB(ctx)).SysDictType
	dictType, err := q.WithContext(ctx).Where(q.DictType.Eq(typeCode)).First()
	if err != nil {
		return nil, err
	}
	return dictType, nil
}

// Add implements base.DictTypeRepo
func (r *dictTypeRepo) Add(ctx context.Context, dictType *model.SysDictType) error {
	err := query.Use(r.db.DB(ctx)).SysDictType.WithContext(ctx).Create(dictType)
	if err != nil {
		r.log.Error("Add dictType failed", zap.Error(err))
		return err
	}
	return nil
}

// Update implements base.DictTypeRepo
func (r *dictTypeRepo) Update(ctx context.Context, dictType *model.SysDictType) error {
	q := query.Use(r.db.DB(ctx)).SysDictType
	rowsAffected, err := q.WithContext(ctx).Where(q.ID.Eq(dictType.ID)).Updates(dictType)
	if err != nil {
		r.log.Error("Update dictType failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// Remove implements base.DictTypeRepo
func (r *dictTypeRepo) Remove(ctx context.Context, id int64) error {
	q := query.Use(r.db.DB(ctx)).SysDictType
	rowsAffected, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	if err != nil {
		r.log.Error("Remove dictType failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// GetById implements base.DictTypeRepo
func (r *dictTypeRepo) GetById(ctx context.Context, id int64) (*model.SysDictType, error) {
	q := query.Use(r.db.DB(ctx)).SysDictType
	dictType, err := q.WithContext(ctx).Where(q.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return dictType, nil
}

// DictTypeList implements base.DictTypeRepo
func (r *dictTypeRepo) DictTypeList(ctx context.Context, pageNum, pageSize int32, dictType, dictName string) ([]*model.SysDictType, int64, error) {
	offset := (pageNum - 1) * pageSize
	dict := query.Use(r.db.DB(ctx)).SysDictType
	q := dict.WithContext(ctx)

	if dictType != "" {
		q = q.Where(dict.DictType.Like("%" + dictType + "%"))
	}
	if dictName != "" {
		q = q.Where(dict.DictName.Like("%" + dictName + "%"))
	}
	dictTypes, count, err := q.FindByPage(int(offset), int(pageSize))
	if err != nil {
		return nil, 0, err
	}
	return dictTypes, count, nil
}

// DictTypeById implements base.DictTypeRepo
func (r *dictTypeRepo) DictTypeById(ctx context.Context, id int64) (*model.SysDictType, error) {
	q := query.Use(r.db.DB(ctx)).SysDictType
	dictType, err := q.WithContext(ctx).Where(q.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return dictType, nil
}

// AddDictType implements base.DictTypeRepo
func (r *dictTypeRepo) AddDictType(ctx context.Context, dictType *model.SysDictType) error {
	err := query.Use(r.db.DB(ctx)).SysDictType.WithContext(ctx).Create(dictType)
	if err != nil {
		r.log.Error("Add dict type failed", zap.Error(err))
		return err
	}
	return nil
}

// UpdateDictType implements base.DictTypeRepo
func (r *dictTypeRepo) UpdateDictType(ctx context.Context, dictType *model.SysDictType) error {
	q := query.Use(r.db.DB(ctx)).SysDictType
	rowsAffected, err := q.WithContext(ctx).Where(q.ID.Eq(dictType.ID)).Updates(dictType)
	if err != nil {
		r.log.Error("Update dict type failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// DeleteDictType implements base.DictTypeRepo.
func (r *dictTypeRepo) DeleteDictType(ctx context.Context, id int64) error {
	q := query.Use(r.db.DB(ctx))
	u := q.SysDictType

	// 删除字典类型
	rowsAffected, err := u.WithContext(ctx).Where(u.ID.Eq(id)).Delete()
	if err != nil {
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}

	return nil
}
