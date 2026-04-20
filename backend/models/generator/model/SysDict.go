/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:13:32
 * @Description: 字典，前端各种状态枚举的存储。
**/
package model

import "github.com/example/aichat/backend/models"

// "dictCode": "5",
//
//	"dictSort": 2,
//	"dictLabel": "隐藏",
//	"dictValue": "1",
//	"dictType": "sys_show_hide",
//	"cssClass": "",
//	"listClass": "danger",
//	"isDefault": false,
//	"status": models.StatusDisabled,
//	"remark": "隐藏菜单",
//	"createTime": 1684048781000
type SysDict struct {
	models.Model
	DictCode  string        `json:"dictCode" gorm:"column:dict_code;type:varchar(64)" db:"dict_code"`
	DictSort  int           `json:"dictSort" gorm:"column:dict_sort;type:int" db:"dict_sort"`
	DictLabel string        `json:"dictLabel" gorm:"column:dict_label;type:varchar(64)" db:"dict_label"`
	DictValue string        `json:"dictValue" gorm:"column:dict_value;type:varchar(64)" db:"dict_value"`
	DictType  string        `json:"dictType" gorm:"column:dict_type;type:varchar(64)" db:"dict_type"`
	CssClass  string        `json:"cssClass" gorm:"column:css_class;type:varchar(64)" db:"css_class"`
	ListClass string        `json:"listClass" gorm:"column:list_class;type:varchar(64)" db:"list_class"`
	IsDefault bool          `json:"isDefault" gorm:"column:is_default;type:bool;default:false" db:"is_default"`
	Status    models.Status `json:"status" gorm:"column:status;type:smallint;default:2" db:"status"`
	Remark    string        `json:"remark" gorm:"column:remark;type:varchar(255)" db:"remark"`
}

func (*SysDict) TableName() string {
	return "sys_dict"
}

//	export interface DictType {
//	  createTime: string;
//	  dictId: number;
//	  dictName: string;
//	  dictType: string;
//	  remark: string;
//	  status: string;
//	}
type SysDictType struct {
	models.Model
	DictType string        `json:"dictType" gorm:"column:dict_type;type:varchar(64)" db:"dict_type"`
	DictName string        `json:"dictName" gorm:"column:dict_name;type:varchar(64)" db:"dict_name"`
	Remark   string        `json:"remark" gorm:"column:remark;type:varchar(255)" db:"remark"`
	Status   models.Status `json:"status" gorm:"column:status;type:smallint;default:2" db:"status"`
}

func (*SysDictType) TableName() string {
	return "sys_dict_type"
}
