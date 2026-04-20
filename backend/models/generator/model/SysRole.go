/* *
 * @Author: chengjiang
 * @Date: 2025-11-17 14:32:54
 * @Description: 角色表
**/
package model

type Role_Type uint8

const (
	Role_Type_Admin Role_Type = iota + 1 // 管理员角色
	Role_Type_User                       // 用户角色
)
