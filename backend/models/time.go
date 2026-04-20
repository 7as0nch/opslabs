/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:58:05
 * @Description:
**/
package models

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)


type Duration time.Duration

// Shrink will decrease the duration by comparing with context's timeout duration
// and return new timeout\context\CancelFunc.
func (d Duration) Shrink(c context.Context) (Duration, context.Context, context.CancelFunc) {
    if deadline, ok := c.Deadline(); ok {
        if ctimeout := time.Until(deadline); ctimeout < time.Duration(d) {
            // deliver small timeout
            return Duration(ctimeout), c, func() {}
        }
    }
    ctx, cancel := context.WithTimeout(c, time.Duration(d))
    return d, ctx, cancel
}

// JSONTime 可在JSON中序列化的时间类型
type Time struct {
    time.Time
}

// MarshalJSON on JSONTime format Time field with %Y-%m-%d %H:%M:%S
func (m Time) MarshalJSON() ([]byte, error) {
    var zeroTime time.Time
    if m.Time.UnixNano() == zeroTime.UnixNano() {
        return []byte("\"\""), nil
    }
    formatted := fmt.Sprintf("\"%s\"", m.Format("2006-01-02 15:04:05"))
    return []byte(formatted), nil
}

// RedisArg 支持redis日期的序列化
func (m Time) RedisArg() interface{} {
    var zeroTime time.Time
    if m.Time.UnixNano() == zeroTime.UnixNano() {
        return []byte("\"\"")
    }
    formatted := m.Format("2006-01-02 15:04:05")
    return []byte(formatted)
}

// RedisScan 支持redis反序列化
func (m *Time) RedisScan(src interface{}) error {
    bs, ok := src.([]byte)
    if !ok {
        return fmt.Errorf("expected []byte, got %T", bs)
    }

    date := strings.ReplaceAll(string(bs), "\"", "")
    tempTime, err := time.ParseInLocation("2006-01-02 15:04:05", date, time.Local)

    if err != nil {
        if date == "" {
            m.Time = time.Now().AddDate(-10, 0, 0)
        }
        t, err := strconv.ParseInt(date, 0, 64)
        if err != nil && date != "" {
            return err
        }
        temp := t / 1000

        m.Time = time.Unix(temp, 0)
    } else {
        m.Time = tempTime
    }
    return nil
}

// UnmarshalJSON JSON反序列接口的实现
func (m *Time) UnmarshalJSON(data []byte) error {

    date := strings.ReplaceAll(string(data), "\"", "")
    var tempTime time.Time
    var err error

    tempTime, err = time.ParseInLocation("2006-01-02 15:04:05", date, time.Local)
    if err != nil {
        tempTime, err = time.ParseInLocation("2006-01-02 15:04:05.000", date, time.Local)
    }
    if err != nil {
        if date == "" {
            m = nil
            return nil
        }
        t, err := strconv.ParseInt(date, 0, 64)
        if err != nil && date != "" {
            return fmt.Errorf("can not parse date %s", string(data))
        }
        temp := t / 1000

        m.Time = time.Unix(temp, 0)
    } else {
        m.Time = tempTime
    }

    return nil
}

// Value insert timestamp into mysql need this function.
func (m Time) Value() (driver.Value, error) {
    var zeroTime time.Time
    if m.Time.UnixNano() == zeroTime.UnixNano() {
        return nil, nil
    }
    return m.Time, nil
}

func (m Time) RedisValue() string {
    return m.Time.Format("2006-01-02 15:04:05")
}

// String 返回JSONTime的字符串格式 YYYY-MM-DD HH:MM:SS
func (m Time) String() string {
    if !m.Time.IsZero() {
        return m.Time.Format("2006-01-02 15:04:05")
    }
    return ""
}

func (m Time) Date() string {
    if !m.Time.IsZero() {
        return m.Time.Format("2006-01-02")
    }
    return ""
}

// Timestamp 返回时间戳格式的字符串
func (m Time) Timestamp() string {
    if !m.Time.IsZero() {
        return m.Time.Format("2006-01-02 15:04:05.000")
    }
    return ""
}

// Scan valueof time.Time
func (m *Time) Scan(v interface{}) error {
    if v == nil {
        return nil
    }
    value, ok := v.(time.Time)
    tm, err := time.ParseInLocation(time.ANSIC, value.Format(time.ANSIC), time.Local)
    if err != nil {
        return err
    }
    if ok {
        *m = Time{Time: tm}
        return nil
    }
    return fmt.Errorf("can not convert %v to timestamp", v)
}

// ToJSONTime 由Time生成NullTime
func ToTime(m time.Time) Time {
    return Time{Time: m}
}

// StrToJSONTime 字符串转换为JSONTime layout为""时默认使用2006-01-02 15:04:05格式 其他时候使用layout的format
func StrToJSONTime(source string, layout string) (Time, error) {
    var times time.Time
    var err error
    if layout == "" {
        times, err = time.ParseInLocation("2006-01-02 15:04:05", source, time.Local)
    } else {
        times, err = time.ParseInLocation(layout, source, time.Local)
    }
    if err != nil {
        return Time{Time: times}, fmt.Errorf("can not parse time %s", source)
    }
    jsontime := ToTime(times)

    return jsontime, nil
}

// Now 当前时间的JSONTime格式
func Now() Time {
    return ToTime(time.Now())
}

func GetStartDateTime(t time.Time) Time {
    return Time{
        Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()),
    }
}

func GetEndDateTime(t time.Time) Time {
    return Time{
        Time: time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location()),
    }
}

