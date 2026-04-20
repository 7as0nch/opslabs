/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 字典类型业务逻辑层
**/
package base

import (
	"context"

	"github.com/example/aichat/backend/models/generator/model"
)

type DictTypeRepo interface {
	DictTypeList(ctx context.Context, pageNum, pageSize int32, dictType, dictName string) ([]*model.SysDictType, int64, error)
	DictTypeById(ctx context.Context, id int64) (*model.SysDictType, error)
	AddDictType(ctx context.Context, dictType *model.SysDictType) error
	UpdateDictType(ctx context.Context, dictType *model.SysDictType) error
	DeleteDictType(ctx context.Context, id int64) error
}

type DictTypeUseCase struct {
	repo     DictTypeRepo
	dataRepo DictDataRepo
	tm       Transaction
}

func NewDictTypeUseCase(repo DictTypeRepo, dataRepo DictDataRepo, tm Transaction) *DictTypeUseCase {
	return &DictTypeUseCase{
		repo:     repo,
		dataRepo: dataRepo,
		tm:       tm,
	}
}

// DictTypeList 获取字典类型列表
func (uc *DictTypeUseCase) DictTypeList(ctx context.Context, pageNum, pageSize int32, dictType, dictName string) ([]*model.SysDictType, int64, error) {
	return uc.repo.DictTypeList(ctx, pageNum, pageSize, dictType, dictName)
}

// DictTypeById 根据ID获取字典类型
func (uc *DictTypeUseCase) DictTypeById(ctx context.Context, id int64) (*model.SysDictType, error) {
	return uc.repo.DictTypeById(ctx, id)
}

// AddDictType 添加字典类型
func (uc *DictTypeUseCase) AddDictType(ctx context.Context, dictType *model.SysDictType) error {
	return uc.repo.AddDictType(ctx, dictType)
}

// UpdateDictType 更新字典类型
func (uc *DictTypeUseCase) UpdateDictType(ctx context.Context, dictType *model.SysDictType) error {
	return uc.repo.UpdateDictType(ctx, dictType)
}

// DeleteDictType 删除字典类型
func (uc *DictTypeUseCase) DeleteDictType(ctx context.Context, id int64) error {
	return uc.tm.InTx(ctx, func(ctx context.Context) error {
		// 1. Get DictType to get the type code
		dictType, err := uc.repo.DictTypeById(ctx, id)
		if err != nil {
			return err
		}
		// 2. Delete associated DictData
		if err := uc.dataRepo.DeleteByDictType(ctx, dictType.DictType); err != nil {
			return err
		}
		// 3. Delete DictType
		err = uc.repo.DeleteDictType(ctx, id)
		if err != nil {
			return err
		}
		return nil
	})
}
