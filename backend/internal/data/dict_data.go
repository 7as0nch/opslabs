/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 字典数据数据访问层
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

type dictDataRepo struct {
	db    db.DataRepo
	log   *zap.Logger
	query *query.Query // 存储预编译的查询实例，避免重复获取DB连接
}

func NewDictDataRepo(db db.DataRepo, log *zap.Logger) base.DictDataRepo {
	// 单例模式：复用查询实例，避免频繁获取数据库连接
	return &dictDataRepo{
		db:    db,
		log:   log,
		query: query.Use(db.GetDB()),
	}
}

// DictDataList implements base.DictDataRepo
func (r *dictDataRepo) DictDataList(ctx context.Context, pageNum, pageSize int32, dictDataLabel, dictType string) ([]*model.SysDict, int64, error) {
	offset := (pageNum - 1) * pageSize
	u := r.query.SysDict.WithContext(ctx)
	if dictDataLabel != "" {
		u = u.Where(r.query.SysDict.DictLabel.Like("%" + dictDataLabel + "%"))
	}
	if dictType != "" {
		u = u.Where(r.query.SysDict.DictType.Eq(dictType))
	}
	dicts, count, err := u.FindByPage(int(offset), int(pageSize))
	if err != nil {
		return nil, 0, err
	}
	return dicts, count, nil
}

// DictDataListByType implements base.DictDataRepo
func (r *dictDataRepo) DictDataListByType(ctx context.Context, dictType string) ([]*model.SysDict, error) {
	dicts, err := r.query.SysDict.WithContext(ctx).Where(r.query.SysDict.DictType.Eq(dictType)).Find()
	if err != nil {
		return nil, err
	}
	return dicts, nil
}

// DictDataById implements base.DictDataRepo
func (r *dictDataRepo) DictDataById(ctx context.Context, id int64) (*model.SysDict, error) {
	dict, err := r.query.SysDict.WithContext(ctx).Where(r.query.SysDict.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}
	return dict, nil
}

// AddDictData implements base.DictDataRepo
func (r *dictDataRepo) AddDictData(ctx context.Context, dictData *model.SysDict) error {
	err := r.query.SysDict.WithContext(ctx).Create(dictData)
	if err != nil {
		r.log.Error("Add dictData failed", zap.Error(err))
		return err
	}
	return nil
}

// UpdateDictData implements base.DictDataRepo
func (r *dictDataRepo) UpdateDictData(ctx context.Context, dictData *model.SysDict) error {
	rowsAffected, err := r.query.SysDict.WithContext(ctx).Where(r.query.SysDict.ID.Eq(dictData.ID)).Updates(dictData)
	if err != nil {
		r.log.Error("Update dictData failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// DeleteDictData implements base.DictDataRepo
func (r *dictDataRepo) DeleteDictData(ctx context.Context, id int64) error {
	rowsAffected, err := r.query.SysDict.WithContext(ctx).Where(r.query.SysDict.ID.Eq(id)).Delete()
	if err != nil {
		r.log.Error("Delete dictData failed", zap.Error(err))
		return err
	}
	if rowsAffected.RowsAffected == 0 {
		return nil
	}
	return nil
}

// DeleteByDictType implements base.DictDataRepo
func (r *dictDataRepo) DeleteByDictType(ctx context.Context, dictType string) error {
	tx := query.Use(r.db.DB(ctx))
	_, err := tx.SysDict.WithContext(ctx).Where(tx.SysDict.DictType.Eq(dictType)).Delete()
	if err != nil {
		r.log.Error("DeleteByDictType failed", zap.Error(err))
		return err
	}
	return nil
}
