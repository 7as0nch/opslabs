/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: Opslabs 服务层的小配置(终端 URL / 空闲超时),由 server 层
 *               从 conf.yaml 解析后注入,避免 service 反向依赖 conf/server
**/
package opslabs

import "time"

// ServiceOptions service 层需要的最小配置集合
type ServiceOptions struct {
	// TerminalHost ttyd 外部访问主机,注入到 terminalURL 模板的 {host}
	TerminalHost string
	// TerminalURLTemplate 终端 URL 模板,支持 {host}/{port} 两个占位符
	// 例子:"http://{host}:{port}/"
	TerminalURLTemplate string
	// DefaultIdleTimeout 无场景覆盖时的默认空闲超时
	DefaultIdleTimeout time.Duration
}

// DefaultServiceOptions Week 1 开发默认值
func DefaultServiceOptions() *ServiceOptions {
	return &ServiceOptions{
		TerminalHost:        "localhost",
		TerminalURLTemplate: "http://{host}:{port}/",
		DefaultIdleTimeout:  30 * time.Minute,
	}
}
