// Package strings @author <cheng jiang>
// @date 2022/12/30
// @note
package strings

import (
	"reflect"
	"strconv"
	"strings"
)

// string:

const (
	TypeError string = "[Type Error !]Now the method only string or int64, make sure your type is right."
)

// StrBuilder the string builder
type StrBuilder struct {
	buf strings.Builder
	len int64
}

func NewStrBuilder(t ...any) *StrBuilder {
	var sb = new(StrBuilder)
	for _, v := range t {
		sb.StrAppend(v)
	}
	return sb
}

// StrAppend 大量字符串的拼接：like the StringBuilder(Java).append(Object).
func (sb *StrBuilder) StrAppend(t any) *StrBuilder {
	type_t := reflect.TypeOf(t)
	value_t := reflect.ValueOf(t)
	if type_t.Kind() == reflect.String {
		sb.buf.WriteString(value_t.String())
	} else if type_t.Kind() == reflect.Int64 {
		sb.buf.WriteString(strconv.FormatInt(value_t.Int(), 10))
	} else {
		panic(TypeError)
	}
	return sb
}
func (sb *StrBuilder) ToString() string {
	return sb.buf.String()
}

func (sb *StrBuilder) Len() int64 {
	sb.len = int64(sb.buf.Len())
	return sb.len
}

// IsEmpty @Author cheng jiang
// @Description // for string, isEmpty then true,others to false.
// @Date 10:00 2023/1/9
// @Param
// @return
func IsEmpty(s string) bool {
	if s == "" {
		return true
	}
	return false
}
func IsNotEmpty(s string) bool {
	return !IsEmpty(s)
}
