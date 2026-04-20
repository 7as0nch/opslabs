/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 字典数据业务逻辑层
**/
package base

import (
	"context"

	"github.com/example/aichat/backend/models/generator/model"
)

type DictDataRepo interface {
	DictDataList(ctx context.Context, pageNum, pageSize int32, dictDataLabel, dictType string) ([]*model.SysDict, int64, error)
	DictDataListByType(ctx context.Context, dictType string) ([]*model.SysDict, error)
	DictDataById(ctx context.Context, id int64) (*model.SysDict, error)
	AddDictData(ctx context.Context, dictData *model.SysDict) error
	UpdateDictData(ctx context.Context, dictData *model.SysDict) error
	DeleteDictData(ctx context.Context, id int64) error
	DeleteByDictType(ctx context.Context, dictType string) error
}

type DictDataUseCase struct {
	repo DictDataRepo
}

func NewDictDataUseCase(repo DictDataRepo) *DictDataUseCase {
	return &DictDataUseCase{
		repo: repo,
	}
}

// DictDataList 获取字典数据列表
func (uc *DictDataUseCase) DictDataList(ctx context.Context, pageNum, pageSize int32, dictDataLabel, dictType string) ([]*model.SysDict, int64, error) {
	return uc.repo.DictDataList(ctx, pageNum, pageSize, dictDataLabel, dictType)
}

// DictDataListByType 根据类型获取字典数据列表
func (uc *DictDataUseCase) DictDataListByType(ctx context.Context, dictType string) ([]*model.SysDict, error) {
	return uc.repo.DictDataListByType(ctx, dictType)
}

// DictDataById 根据ID获取字典数据
func (uc *DictDataUseCase) DictDataById(ctx context.Context, id int64) (*model.SysDict, error) {
	return uc.repo.DictDataById(ctx, id)
}

// AddDictData 添加字典数据
func (uc *DictDataUseCase) AddDictData(ctx context.Context, dictData *model.SysDict) error {
	return uc.repo.AddDictData(ctx, dictData)
}

// UpdateDictData 更新字典数据
func (uc *DictDataUseCase) UpdateDictData(ctx context.Context, dictData *model.SysDict) error {
	return uc.repo.UpdateDictData(ctx, dictData)
}

// DeleteDictData 删除字典数据
func (uc *DictDataUseCase) DeleteDictData(ctx context.Context, id int64) error {
	return uc.repo.DeleteDictData(ctx, id)
}
