package models

import (
	"github.com/example/aichat/backend/tools"
	"gorm.io/plugin/soft_delete"
)

/* *
 * @Author: chengjiang
 * @Date: 2025-10-04 17:14:27
 * @Description:
**/

type Model struct {
	ID        int64                 `gorm:"column:id;type:bigint;primaryKey" json:"id"`
	CreatedAt Time                  `gorm:"column:created_at;type:time;" json:"created_at"`
	CreatedBy int64                 `gorm:"column:created_by;type:bigint" json:"created_by"` // 创建人
	UpdatedAt Time                  `gorm:"column:updated_at;type:time;" json:"updated_at"`
	UpdatedBy int64                 `gorm:"column:updated_by;type:bigint" json:"updated_by"`                             // 更新人
	IsDeleted soft_delete.DeletedAt `gorm:"column:is_deleted;type:smallint;default:0;softDelete:flag" json:"is_deleted"` // 是否删除0否1是
	DeletedAt Time                  `gorm:"column:deleted_at;type:time" json:"deleted_at"`
	DeletedBy int64                 `gorm:"column:deleted_by;type:bigint" json:"deleted_by"` // 删除人
}

func (m *Model) New() {
	m.ID = tools.GetSFID()
}

type Status uint8

// iota
const (
	Status_Enabled  Status = iota + 1 // 启用
	Status_Disabled                   // 禁用
)

func (s Status) String() string {
	switch s {
	case Status_Enabled:
		return "1"
	case Status_Disabled:
		return "2"
	default:
		return "-"
	}
}

// str to Status
func ToStatus(str string) Status {
	switch str {
	case "1":
		return Status_Enabled
	case "2":
		return Status_Disabled
	default:
		return 0
	}
}

type SystemType uint8

const (
	SystemType_System SystemType = iota + 1 // 系统内置
	SystemType_User                         // 用户自定义
)
