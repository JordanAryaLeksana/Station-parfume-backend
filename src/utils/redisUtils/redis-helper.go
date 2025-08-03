package utils

import (
	"backend/src/config"
	"context"
	"time"
)

func BlacklistToken(token string, duration time.Duration) error {
	return config.RedisClient.Set(context.Background(), token, "blacklisted", duration).Err()
}

func IsTokenBlacklisted(token string) (bool, error) {
	result, err := config.RedisClient.Get(context.Background(), token).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil // Token is not blacklisted
		}
		return false, err 
	}
	return result == "blacklisted", nil
}