// Package utils: blacklist jti token di Redis (logout & rotasi refresh token).
package utils

import (
	"backend/src/config"
	"context"
	"time"
)

const redisTimeout = 2 * time.Second

func key(jti string) string { return "blacklist:" + jti }

// BlacklistJti mencabut token (jti) sampai token itu kedaluwarsa sendiri.
func BlacklistJti(jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // sudah kedaluwarsa, tidak perlu dicatat
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	return config.RedisClient.Set(ctx, key(jti), "revoked", ttl).Err()
}

// IsJtiBlacklisted: error Redis dikembalikan apa adanya; pemanggil harus
// fail-closed (menolak request), bukan meloloskan token.
func IsJtiBlacklisted(jti string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	n, err := config.RedisClient.Exists(ctx, key(jti)).Result()
	return n > 0, err
}

// ConsumeJti menandai jti sebagai terpakai secara ATOMIK (SET NX). true = baru
// dipakai sekarang; false = sudah pernah dipakai / dicabut. Dipakai untuk rotasi
// refresh token: refresh token yang sama tidak bisa dipakai dua kali, termasuk
// oleh dua request bersamaan.
func ConsumeJti(jti string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	return config.RedisClient.SetNX(ctx, key(jti), "used", ttl).Result()
}
