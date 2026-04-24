/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: biz/attempt 门面:重导出 model.OpslabsAttempt + 状态枚举,
 *               避免上层到处 import 到 models/generator/model,同时定义 Repo 接口
**/
package attempt

import (
	"context"

	"github.com/7as0nch/backend/models/generator/model"
)

// Attempt 领域别名,真实结构定义在 models/generator/model/OpslabsAttempt.go
// 使用别名而非包装类型,直接和 gorm-gen 生成的 query 层兼容
type Attempt = model.OpslabsAttempt

// AttemptStatus 状态枚举别名
type AttemptStatus = model.AttemptStatus

// 状态常量透出,biz/data 层可以直接写 attempt.StatusRunning
const (
	StatusRunning    = model.AttemptStatusRunning
	StatusPassed     = model.AttemptStatusPassed
	StatusExpired    = model.AttemptStatusExpired
	StatusTerminated = model.AttemptStatusTerminated
	StatusFailed     = model.AttemptStatusFailed
)

// AttemptRepo 数据访问接口,data 层实现
//
// 语义:
//   - Create/Update 常规 CRUD,错误直接回写
//   - FindByID 命中不到返回 ErrAttemptNotFound(而不是 gorm.ErrRecordNotFound)
//
// 注:ListRunning 已于 Round 6 移除 —— AttemptStore 迁 Redis 后不再需要
// "进程启动时从 DB 回灌内存缓存",Redis 本身就是跨进程共享。
type AttemptRepo interface {
	Create(ctx context.Context, a *Attempt) error
	Update(ctx context.Context, a *Attempt) error
	FindByID(ctx context.Context, id int64) (*Attempt, error)
}
