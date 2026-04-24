/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 场景尝试记录(一次 Start 对应一条),承载容器信息 + 状态机
**/
package model

import (
	"time"

	"github.com/7as0nch/backend/models"
)

const TableNameOpslabsAttempt = "opslabs_attempts"

// OpslabsAttempt 一次场景实战记录
//
// 规约:
//   - ID 复用 models.Model 里的 bigint 雪花 ID,API 层以字符串输出
//   - UserID=0 表示匿名(Week 1 未接入登录,保留向后兼容)
//   - ClientID 前端生成的 uuid(localStorage 持久化),未接入登录前做"谁在做"标识
//     与 UserID 并存:登录接入后 UserID 主导,ClientID 仅用于同一用户多设备区分
//   - ContainerID 是容器运行时返回的 ID,mock 模式下是 32 字符 hex
//   - HostPort 是宿主机映射的 ttyd 端口,Stop 时归还端口池
//   - Status/FinishedAt/DurationMS 构成状态机快照
type OpslabsAttempt struct {
	models.Model
	UserID       int64            `json:"userId" gorm:"column:user_id;type:bigint;default:0;index" db:"user_id"`                      // 用户ID(0 表示匿名)
	ClientID     string           `json:"clientId" gorm:"column:client_id;type:varchar(64);index" db:"client_id"`                     // 前端 clientID(匿名场景做 owner 标识)
	ScenarioSlug string           `json:"scenarioSlug" gorm:"column:scenario_slug;type:varchar(64);index;not null" db:"scenario_slug"` // 场景 slug
	ContainerID  string           `json:"containerId" gorm:"column:container_id;type:varchar(128)" db:"container_id"`                 // 运行时容器 ID
	HostPort     int              `json:"hostPort" gorm:"column:host_port;type:integer;default:0" db:"host_port"`                     // 宿主机 ttyd 端口
	Status       AttemptStatus    `json:"status" gorm:"column:status;type:varchar(16);index;not null" db:"status"`                    // 状态机当前值
	StartedAt    time.Time        `json:"startedAt" gorm:"column:started_at;type:timestamptz;not null" db:"started_at"`               // 开始时间
	LastActiveAt time.Time        `json:"lastActiveAt" gorm:"column:last_active_at;type:timestamptz;index;not null" db:"last_active_at"` // 最近心跳
	FinishedAt   *time.Time       `json:"finishedAt,omitempty" gorm:"column:finished_at;type:timestamptz" db:"finished_at"`           // 结束时间,进行中为 NULL
	DurationMS   *int64           `json:"durationMs,omitempty" gorm:"column:duration_ms;type:bigint" db:"duration_ms"`                // 用时毫秒,进行中为 NULL
	CheckCount   int              `json:"checkCount" gorm:"column:check_count;type:integer;default:0" db:"check_count"`               // 判题触发次数
}

// TableName 指定表名
func (*OpslabsAttempt) TableName() string {
	return TableNameOpslabsAttempt
}

// AttemptStatus 场景尝试的状态枚举
type AttemptStatus string

const (
	AttemptStatusRunning    AttemptStatus = "running"    // 运行中
	AttemptStatusPassed     AttemptStatus = "passed"     // 已通关
	AttemptStatusExpired    AttemptStatus = "expired"    // 空闲超时
	AttemptStatusTerminated AttemptStatus = "terminated" // 手动结束
	AttemptStatusFailed     AttemptStatus = "failed"     // 启动/运行异常失败
)

// IsActive 当前 Attempt 是否还在跑
func (a *OpslabsAttempt) IsActive() bool {
	return a.Status == AttemptStatusRunning
}

// MarkPassed 标记为通关并锁定用时
func (a *OpslabsAttempt) MarkPassed(now time.Time) {
	a.setFinishedStatus(AttemptStatusPassed, now)
}

// MarkTerminated 用户/系统主动结束
func (a *OpslabsAttempt) MarkTerminated(now time.Time) {
	a.setFinishedStatus(AttemptStatusTerminated, now)
}

// MarkExpired 空闲超时被清理
func (a *OpslabsAttempt) MarkExpired(now time.Time) {
	a.setFinishedStatus(AttemptStatusExpired, now)
}

// MarkFailed 启动/exec 异常
func (a *OpslabsAttempt) MarkFailed(now time.Time) {
	a.setFinishedStatus(AttemptStatusFailed, now)
}

func (a *OpslabsAttempt) setFinishedStatus(s AttemptStatus, now time.Time) {
	a.Status = s
	t := now
	a.FinishedAt = &t
	ms := now.Sub(a.StartedAt).Milliseconds()
	a.DurationMS = &ms
}
