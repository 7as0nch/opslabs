package loginprovider

import "github.com/7as0nch/backend/internal/db"

func NewStateCache(repo db.RedisRepo) StateCache {
	return repo
}
