/* *
 * @Author: chengjiang
 * @Date: 2026-03-16 16:42:45
 * @Description: Redis key 命名规范
 *               命名约定:<业务>:<实体>:<维度>:<标识>,冒号分隔,全小写
**/
package consts

const (
	RedisKeyQQState         = "user:auth:oauth:qq:state:%s"
	RedisKeyChatStreamMeta  = "chat:stream:%d:meta"
	RedisKeyChatStreamDelta = "chat:stream:%d:delta"
)

// ------------------------------------------------------------
// opslabs AttemptStore(Round 6 迁 Redis)
// ------------------------------------------------------------
//
// 数据模型:
//
//   主 key:
//     opslabs:attempt:<id>  (String, JSON 序列化的 OpslabsAttempt)
//     TTL = 2 * idleCutoff,GC 清 + Redis 过期双兜底
//
//   活跃集合:
//     opslabs:attempt:active  (SET of attemptID 字符串)
//     无 TTL,Put 时 SADD,Delete / Terminate 时 SREM
//     Snapshot 走 SMEMBERS active → MGET 主 key
//
//   Owner 反查索引(ClientID 维度 + UserID 维度,支持 FindActive*):
//     opslabs:attempt:owner:client:<clientID>:<slug>  → String("<id>")
//     opslabs:attempt:owner:user:<userID>:<slug>      → String("<id>")
//     TTL 同主 key,命中后再 GET 主 key 取完整数据
//
// 一致性策略:
//   写入:Pipeline 一次打包(主 key + SAdd active + 两条 owner 索引)
//   读取:SMEMBERS 可能先于主 key 过期,MGet 拿到 nil 时当"已失效"处理
//   删除:Pipeline 一次打包(DEL 主 key + SREM active + DEL 两条 owner)
const (
	// RedisKeyAttempt 主数据,%d = attemptID
	RedisKeyAttempt = "opslabs:attempt:%d"
	// RedisKeyAttemptActive 活跃 attemptID 集合,无参数
	RedisKeyAttemptActive = "opslabs:attempt:active"
	// RedisKeyAttemptOwnerClient (clientID, slug) → attemptID 反查索引
	//   %s = clientID, %s = slug
	RedisKeyAttemptOwnerClient = "opslabs:attempt:owner:client:%s:%s"
	// RedisKeyAttemptOwnerUser (userID, slug) → attemptID 反查索引
	//   %d = userID, %s = slug
	RedisKeyAttemptOwnerUser = "opslabs:attempt:owner:user:%d:%s"
)

