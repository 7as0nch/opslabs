// Package tools @author <chengjiang@buffalo-robot.com>
// @date 2023/2/20
// @note
package tools

import "time"

func Time2String(ti *time.Time) string {
	if ti != nil {
		return ti.Format("2006-01-02 15:04:05")
		//return ti.Format(time.DateTime)
	} else {
		return ""
	}
}
