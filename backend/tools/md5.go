// Package tools @author <chengjiang@buffalo-robot.com>
// @date 2023/2/16
// @note
package tools

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

const SignalKey = "chengjiang@stu.cdu.edu.cn"

func Check(newPwd, oldPwd string) bool {
	return strings.EqualFold(Encode(newPwd), oldPwd)
}

func Encode(data string) string {
	hash := md5.New()
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum([]byte(SignalKey)))
}
