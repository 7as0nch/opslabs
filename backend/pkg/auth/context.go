// Package auth @author <chengjiang@buffalo-robot.com>
// @date 2023/1/10
// @note
package auth

import (
	"context"
)

type ContextKey int

const (
	_ ContextKey = iota
	UserId
	UserName
	UserPhone
	ClientID
)

type Context getContextKeyimpl

type getContextKeyimpl struct{}

func (c *getContextKeyimpl) Get(ctx context.Context, key ContextKey) any {
	return ctx.Value(key)
}
