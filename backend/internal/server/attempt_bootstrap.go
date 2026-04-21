/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 进程启动时从 DB 回灌 running attempts 到内存 store
 *               实现 kratos.transport.Server,和 http/grpc 一同注册
 *               Start 阶段同步执行 ListRunning;失败只记日志,不阻塞主服务
**/
package server

import (
	"context"

	"github.com/7as0nch/backend/internal/biz/attempt"
	"go.uber.org/zap"
)

// AttemptBootstrapper 启动时从 DB 恢复内存缓存
// 作为 kratos transport.Server 的一员,保证它在 grpc/http 开放前完成
type AttemptBootstrapper struct {
	uc  *attempt.AttemptUsecase
	log *zap.Logger
}

// NewAttemptBootstrapper 构造
func NewAttemptBootstrapper(uc *attempt.AttemptUsecase, log *zap.Logger) *AttemptBootstrapper {
	return &AttemptBootstrapper{uc: uc, log: log}
}

// Start 同步恢复;kratos Start 是串行的,DB 回灌完才会开放 http/grpc
func (b *AttemptBootstrapper) Start(ctx context.Context) error {
	if err := b.uc.RestoreRunning(ctx); err != nil {
		// 吞掉错误,避免启动失败;异常 attempt 下一轮 reaper 会清理
		b.log.Error("restore running attempts failed", zap.Error(err))
	}
	return nil
}

// Stop noop
func (b *AttemptBootstrapper) Stop(ctx context.Context) error {
	return nil
}
