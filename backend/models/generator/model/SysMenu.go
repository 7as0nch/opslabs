/* *
 * @Author: chengjiang
 * @Date: 2025-11-16 01:34:49
 * @Description:
**/
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/example/aichat/backend/models"
)

const TableNameSysMenu = "sys_menu"

type Meta struct {
	Title   string `json:"title" db:"title"`
	Icon    string `json:"icon" db:"icon"`
	NoCache bool   `json:"noCache" db:"no_cache"`
}

type MenuType uint8

const (
	MenuTypeDir    MenuType = iota + 1 // 目录
	MenuTypeMenu                       // 菜单
	MenuTypeButton                     // 按钮
)

// Tostring implements fmt.Stringer.
func (m MenuType) String() string {
	switch m {
	case MenuTypeDir:
		return "M"
	case MenuTypeMenu:
		return "C"
	case MenuTypeButton:
		return "F"
	default:
		return "-"
	}
}

// ToMenuType 将字符串转换为 MenuType 枚举值
func ToMenuType(str string) MenuType {
	switch str {
	case "M":
		return MenuTypeDir
	case "C":
		return MenuTypeMenu
	case "F":
		return MenuTypeButton
	default:
		return 0
	}
}

// Menu 菜单结构体（树形结构）
type SysMenu struct {
	models.Model
	Name       string        `json:"name" db:"name"`
	Path       string        `json:"path" db:"path"`
	Hidden     bool          `json:"hidden" db:"hidden"`
	Redirect   string        `json:"redirect" db:"redirect"` // 是否重定向
	Component  string        `json:"component" db:"component"`
	AlwaysShow bool          `json:"alwaysShow" db:"always_show"`
	Meta       *Meta         `json:"meta" gorm:"meta;type:json" db:"meta"`
	ParentID   int64         `json:"parentId" db:"parent_id"`
	Sort       int           `json:"sort" db:"sort"`
	Type       MenuType      `json:"type" gorm:"type:smallint" db:"type"`
	Remark     string        `json:"remark" gorm:"remark" db:"remark"`
	PermsCode  string        `json:"permsCode" gorm:"perms_code;type:varchar(20)" db:"perms_code"`
	Status     models.Status `json:"status" gorm:"status;type:smallint;default:1" db:"status"`
	Children   []*SysMenu    `json:"children" gorm:"-" db:"-"`
}

// TableName 指定表名
func (*SysMenu) TableName() string {
	return TableNameSysMenu
}

// MenuToTree 将菜单列表转换为树形结构
func MenuToTree(menus []*SysMenu, parentID int64) []*SysMenu {
	var tree []*SysMenu
	for _, menu := range menus {
		if menu.ParentID == parentID {
			// 递归查找子菜单
			menu.Children = MenuToTree(menus, menu.ID)
			tree = append(tree, menu)
		}
	}
	// order by sort field
	sort.SliceStable(tree, func(i, j int) bool {
		return tree[i].Sort < tree[j].Sort
	})
	return tree
}

func (m Meta) Value() (driver.Value, error) {
	if m == (Meta{}) {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan 实现sql.Scanner接口，用于从数据库值扫描到结构体
func (m *Meta) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		return fmt.Errorf("cannot scan %T into Meta", value)
	}
}
