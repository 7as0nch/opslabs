// Package tools @author <chengjiang@buffalo-robot.com>
// @date 2023/2/16
// @note
package tools

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/example/aichat/backend/pkg/lib"
)

// 支持 2 ^ 8 - 1 台机器
// 每一个毫秒支持 2 ^ 9 - 1 个不同的id
const (
	workerIdBitsMoveLen = uint(8)
	maxWorkerId         = int64(-1 ^ (-1 << workerIdBitsMoveLen))
	timerIdBitsMoveLen  = uint(17)
	maxNumId            = int64(-1 ^ (-1 << 9))
)

// Worker1 定义一个woker工作节点所需要的基本参数
type Worker1 struct {
	mu        sync.Mutex // 添加互斥锁 确保并发安全
	workerId  int64      // 机器编码
	timestamp int64      // 记录时间戳
	number    int64      // 当前毫秒已经生成的id序列号(从0开始累加) 1毫秒内最多生成4096个ID
}

// NewWorker1 初始化ID生成结构体
// workerId 机器的编号
func NewWorker1(workerId int64) *Worker1 {
	if workerId > maxWorkerId {
		panic("workerId 不能大于最大值")
	}
	return &Worker1{workerId: workerId, timestamp: 0, number: 0}
}

// GetId 生成id 的方法用于生成唯一id
func (w *Worker1) GetId() int64 {
	epoch := int64(1613811738) // 设置为去年今天的时间戳...因为位数变了后,几百年都用不完,,实际可以设置上线日期的
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now().UnixMilli() // 获得现在对应的时间戳
	if now < w.timestamp {
		// 当机器出现时钟回拨时报错
		panic("Clock moved backwards.  Refusing to generate id for %d milliseconds")
	}
	if w.timestamp == now {
		w.number++
		if w.number > maxNumId { //此处为最大节点ID,大概是2^9-1 511条,
			for now <= w.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		w.number = 0
		w.timestamp = now // 将机器上一次生成ID的时间更新为当前时间
	}
	ID := int64((now-epoch)<<timerIdBitsMoveLen | (w.workerId << workerIdBitsMoveLen) | (w.number))
	return ID
}

func testGetId() {
	worker := NewWorker1(55)
	arr := make([]int64, 0, 100)

	for i := 0; i < 100; i++ {
		arr = append(arr, worker.GetId())
	}
	fmt.Printf("%+v\n", arr)
}


var Node *snowflake.Node

func init() {
	// 1. 【修改这里】设置起始时间 (Epoch)
    // 必须在 NewNode 之前设置。
    // 这里设置为 2025-01-01 00:00:00 UTC 的毫秒时间戳
    // 你也可以直接写死一个 int64 数字，比如 1735689600000
    // st := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    // snowflake.Epoch = st.UnixMilli()
	//
    ip := lib.GetIP()
    var seed int
    var err error
    if ip == "" {
        seed = 1
    } else {
        arr := strings.Split(ip, ".")
        lastOne := arr[3]
        seed, err = strconv.Atoi(lastOne)
        if err != nil {
            panic(err)
        }
    }
    idGenerator, err := snowflake.NewNode(int64(seed))
    if err != nil {
        panic(err)
    }
    Node = idGenerator
}

func GetSFID() int64 {
    return Node.Generate().Int64()
}
