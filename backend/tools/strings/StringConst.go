package strings

import "errors"

const (
	EmptyStr = ""
)

var (
	CreateError = errors.New("创建失败")
	UpdateError = errors.New("更新失败")
	DeleteError = errors.New("删除失败")
)
