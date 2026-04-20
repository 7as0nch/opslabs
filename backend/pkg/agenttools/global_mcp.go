/* *
 * @Author: chengjiang
 * @Date: 2025-10-06 19:45:21
 * @Description:
**/
package agenttools

import (
	"context"
	"time"

)

type CurrentTimeParams struct {
	TimeZone string `json:"time_zone" jsonschema_description:"time zone, default the Asia/Shanghai"`
}

func getCurrentTimeFunc(_ context.Context, params *CurrentTimeParams) (string, error) {
	timeZone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(timeZone).Format(time.DateTime)
	return `{"current_time": "` + now + `"}`, nil
}