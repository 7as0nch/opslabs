// Package test @author <chengjiang@buffalo-robot.com>
// @date 2023/1/6
// @note
package test

import (
	"testing"
	"time"

	myStrings "github.com/example/aichat/backend/tools/strings"
)

func TestStrBuilder(t *testing.T) {
	var sb = myStrings.NewStrBuilder("cheng jiang")
	sb.StrAppend(", i make the string append easily.")
	t.Log(sb.ToString())
	sb.StrAppend(" my age is: ").StrAppend(int64(100000000))
	t.Log(sb.ToString(), sb.Len())
	// 测试一下分别直接拼接和使用builder的性能。
	var sb1 = myStrings.NewStrBuilder("")
	start1 := time.Now()
	for i := 0; i < 100000; i++ {
		sb1.StrAppend("hhh")
	}
	end1 := time.Now()
	start2 := time.Now()
	var str = ""
	for i := 0; i < 100000; i++ {
		str += "hhh"
	}
	end2 := time.Now()
	t.Log("builder: ", end1.Sub(start1), ", string: ", end2.Sub(start2))
	// the output:
	// builder:  1.0368ms , string:  1.2644461s
}
