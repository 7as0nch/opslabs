package agenttools

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	emcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/mark3labs/mcp-go/client"
)

/* *
 * @Author: chengjiang
 * @Date: 2025-10-05 10:11:52
 * @Description:
**/


func GetGlobalTools() []tool.BaseTool {
	getLocalTimeFunc, err := utils.InferTool(
		"get_current_time", 
		"获取当前时间", getCurrentTimeFunc)
	if err != nil {
		panic(err)
	}
	return []tool.BaseTool{getLocalTimeFunc}
}

func GetBussinessTools() []tool.BaseTool {
	queryUserListFunc, err := utils.InferTool(

		"query_user_list", 
		"根据时间范围（有可能是前三天：表示今天现在这个时间到前面三天，这个不用掉工具，直接计算），查询或生成用户报表，时间：Asia/Shanghai格式", queryUserListFunc)
	if err != nil {
		panic(err)
	}
	// GetChannelRoi
	getChannelRoiFunc, err := utils.InferTool(
		"get_channel_roi", 
		"根据渠道ID和时间范围，查询渠道ROI", GetChannelRoi)
	if err != nil {
		panic(err)
	}
	return []tool.BaseTool{
		utils.WrapInvokableToolWithErrorHandler(queryUserListFunc, func(ctx context.Context, err error) string {
			return err.Error()
		}),
		utils.WrapInvokableToolWithErrorHandler(getChannelRoiFunc, func(ctx context.Context, err error) string {
			return err.Error()
		}),
	}
}

// mcpTools：查询pg数据库内容。
func GetTools() []tool.BaseTool {
	queryUserListFunc, err := utils.InferTool(

		"query_user_list", 
		"根据时间范围（有可能是前三天：表示今天现在这个时间到前面三天，这个不用掉工具，直接计算），查询或生成用户报表，时间：Asia/Shanghai格式", queryUserListFunc)
	if err != nil {
		panic(err)
	}
	getLocalTimeFunc, err := utils.InferTool(
		"get_current_time", 
		"获取当前时间", getCurrentTimeFunc)
	if err != nil {
		panic(err)
	}
	// GetChannelRoi
	getChannelRoiFunc, err := utils.InferTool(
		"get_channel_roi", 
		"根据渠道ID和时间范围，查询渠道ROI", GetChannelRoi)
	if err != nil {
		panic(err)
	}
	return []tool.BaseTool{
		utils.WrapInvokableToolWithErrorHandler(queryUserListFunc, func(ctx context.Context, err error) string {
			return err.Error()
		}),
		utils.WrapInvokableToolWithErrorHandler(getLocalTimeFunc, func(ctx context.Context, err error) string {
			return err.Error()
		}),
		utils.WrapInvokableToolWithErrorHandler(getChannelRoiFunc, func(ctx context.Context, err error) string {
			return err.Error()
		}),
	}
}

func GetStreamTools() []tool.StreamableTool {
	return []tool.StreamableTool{
		// {},
	}
}

func McpTools() []tool.BaseTool {
	cli, err := client.NewSSEMCPClient("http://localhost:8080/sse")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = cli.Start(ctx)
	if err != nil {
		panic(err)
	}
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Hello World Server",
		Version: "1.0.0",
	}
	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		panic(err)
	}
	tools, err := emcp.GetTools(ctx, &emcp.Config{Cli: cli})
	if err != nil {
		panic(err)
	}
	return tools
}