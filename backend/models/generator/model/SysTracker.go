/* *
 * @Author: chengjiang
 * @Date: 2025-11-16 01:34:49
 * @Description: 埋点跟踪。
 **/
package model

import (
	"github.com/example/aichat/backend/models"
)

const TableNameSysTracker = "sys_tracker"

// SysTracker 埋点跟踪结构体
type SysTracker struct {
	models.Model
	AppId     string      `json:"appId" gorm:"column:app_id;type:varchar(50)" db:"app_id"`              // 应用ID
	DeviceId  string      `json:"deviceId" gorm:"column:device_id;type:varchar(100)" db:"device_id"`    // 设备ID
	UserId    int64       `json:"userId" gorm:"column:user_id;type:bigint" db:"user_id"`                // 用户ID
	Timestamp string      `json:"timestamp" gorm:"column:timestamp;type:varchar(255)" db:"timestamp"`   // 时间戳
	UserAgent string      `json:"userAgent" gorm:"column:user_agent;type:varchar(255)" db:"user_agent"` // 用户代理
	PageUrl   string      `json:"pageUrl" gorm:"column:page_url;type:varchar(255)" db:"page_url"`       // 页面URL
	Type      TrackerType `json:"type" gorm:"column:type;type:varchar(10)" db:"type"`                   // 类型
	Data      string      `json:"data" gorm:"column:data;type:text" db:"data"`                          // 数据
}

// TableName 指定表名
func (*SysTracker) TableName() string {
	return TableNameSysTracker
}

type TrackerType string

// pv, api, button, stay
const (
	TrackerTypePageView TrackerType = "pv"     // 页面访问
	TrackerTypeApi      TrackerType = "api"    // API调用
	TrackerTypeButton   TrackerType = "button" // 按钮点击
	TrackerTypeStay     TrackerType = "stay"   // 停留时间
)
