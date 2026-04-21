package data

import (
	"github.com/7as0nch/backend/internal/biz/base"
	"github.com/7as0nch/backend/internal/db"
)

func NewTransaction(d db.DataRepo) base.Transaction {
	return d
}
