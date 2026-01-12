package internal

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client
var Ctx = context.Background()

func ConnectRedis() {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	RDB = redis.NewClient(&redis.Options{
		Addr:     host + ":6379", // ใช้ host ที่ได้มาต่อด้วย Port
		Password: "",
		DB:       0,
	})
}
