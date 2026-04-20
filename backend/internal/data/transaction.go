package data

import (
	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/internal/db"
)

func NewTransaction(d db.DataRepo) base.Transaction {
	return d
}
