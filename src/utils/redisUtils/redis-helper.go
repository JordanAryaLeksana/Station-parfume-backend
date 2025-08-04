package utils

import (
	"backend/src/config"
	"context"
	"time"
)

func BlacklistJti(jti string, duration time.Duration) error {
	return config.RedisClient.Set(context.Background(), "blacklist:"+jti, "blacklisted", duration).Err()
}

func IsJtiBlacklisted(jti string) (bool, error) {
	result, err := config.RedisClient.Get(context.Background(), "blacklist:"+jti).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil // Jti is not blacklisted
		}
		return false, err 
	}
	return result == "blacklisted", nil
}