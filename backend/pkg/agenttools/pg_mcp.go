/* *
 * @Author: chengjiang
 * @Date: 2025-10-05 10:13:27
 * @Description:
**/
package agenttools

import (
	"context"
	"time"

)

type UserListParams struct {
	// PageNum  int       `json:"page_num,omitempty" jsonschema:"description=page number"`
	// PageSize int       `json:"page_size,omitempty" jsonschema:"description=page size"`
	StartAt string `json:"start_at" jsonschema:"description=start time of the user's createAt in time.DateTime"`
	EndAt   string `json:"end_at" jsonschema:"description=end time of the user's createAt in time.DateTime"`
}

type UserReportParams struct {
	UserID    string    `json:"user_id" jsonschema:"description=user id"`
	UserName  string    `json:"user_name" jsonschema:"description=user name"`
	CreatedAt string    `json:"created_at" jsonschema:"description=the user created at time in time.DateTime"`
	Gender    string    `json:"gender" jsonschema:"description=user gender"`
	Age       int       `json:"age" jsonschema:"description=user age"`
}

type Result struct {
	Data  []*UserReportParams `json:"data" jsonschema:"description=user report params"`
	Total int64               `json:"total" jsonschema:"description=total"`
	Msg   string              `json:"msg" jsonschema:"description=message"`
}

// 时间范围内的用户报表
func queryUserListFunc(_ context.Context, params *UserListParams) (*Result, error) {
	// logs.Infof("invoke tool query_user_list: %+v", params)

	// Tool处理代码
	// ...
	// 检查参数是否给全了
	// 检查时间范围是否合法
	// if params.StartAt.After(params.EndAt) {
	// 	return "", errors.New("start_at must be before end_at")
	// }
	// 返回示例数据:时区为Asia/Shanghai
	timeZone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(timeZone)
	return &Result{
		Data: []*UserReportParams{
			{
				UserID:    "1",
				UserName:  "chengjiang",
				CreatedAt: now.Format(time.DateTime),
				Gender:    "male",
				Age:       18,
			},
			{
				UserID:    "2",
				UserName:  "chenghaiyan",
				CreatedAt: now.Format(time.DateTime),
				Gender:    "female",
				Age:       20,
			},
		},
		Total: 2,
		Msg:   "success",
	}, nil
}



type ChannelRoiParam struct {
	ChannelID int64 `json:"channel_id" jsonschema:"description=渠道id"`
	StartTime string `json:"start_time" jsonschema:"description=开始时间"`
	EndTime   string `json:"end_time" jsonschema:"description=结束时间"`
}
type ChannelRoi struct {
	// 渠道id
	SubChannelID int64 `json:"sub_channel_id" gorm:"type:int;not null" jsonschema:"description=渠道id"`
	// 半流程分发收益
	HalfProfit float64 `json:"half_profit" gorm:"type:decimal(6,2);not null" jsonschema:"description=半流程分发收益"`
	// 半流程分发净收益
	HalfNetProfit float64 `json:"half_net_profit" gorm:"type:decimal(6,2);not null" jsonschema:"description=半流程分发净收益"`
}

func GetChannelRoi(ctx context.Context, param *ChannelRoiParam) ([]*ChannelRoi, error) {
	var channelRois []*ChannelRoi
	data, _, err := NewData()
	if err != nil {
		return nil, err
	}
	err = data.db.WithContext(ctx).Raw(`
		SELECT
			aho.sub_channel_id AS 'sub_channel_id',
			SUM(price) AS 'half_profit',
			SUM(income) AS 'half_net_profit'
		FROM
			api_halfpro_order AS aho
			LEFT JOIN api_halfpro_order_line AS ahol ON aho.id = ahol.order_id
		WHERE
			aho.channel_id = ?
			AND aho.created_at BETWEEN ?
			AND ?
			AND ahol.check_status = 1
			AND ahol.push_status = 1 
		GROUP BY
			aho.sub_channel_id;
	`, param.ChannelID, param.StartTime, param.EndTime).Scan(&channelRois).Error
	if err != nil {
		return nil, err
	}
	return channelRois, nil
}