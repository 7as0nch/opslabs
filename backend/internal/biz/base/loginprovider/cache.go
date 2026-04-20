package loginprovider

import "github.com/example/aichat/backend/internal/db"

func NewStateCache(repo db.RedisRepo) StateCache {
	return repo
}
