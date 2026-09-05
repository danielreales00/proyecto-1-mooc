// Package sessions implementa identity.SessionStore sobre Redis.
//
// La clave se deriva del SHA-256 del token, nunca del token: un volcado de
// Redis no entrega sesiones utilizables (ADR-0006).
package sessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"mooc/backend/internal/modules/identity"
)

type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sess:" + hex.EncodeToString(sum[:])
}

func userKey(userID uuid.UUID) string { return "user_sessions:" + userID.String() }

func (s *Store) Create(ctx context.Context, token string, sess identity.Session, ttl time.Duration) error {
	key := sessionKey(token)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"session_id":   sess.ID.String(),
		"user_id":      sess.UserID.String(),
		"role":         sess.Role,
		"ip":           sess.IP,
		"user_agent":   sess.UserAgent,
		"created_at":   sess.CreatedAt.Unix(),
		"last_seen_at": sess.LastSeenAt.Unix(),
		"expires_at":   sess.ExpiresAt.Unix(),
	})
	pipe.Expire(ctx, key, ttl)
	// Índice inverso: permite revocar todas las sesiones de un usuario de un
	// golpe cuando entre la administración (RF-02).
	pipe.SAdd(ctx, userKey(sess.UserID), key)
	pipe.Expire(ctx, userKey(sess.UserID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("crear sesión en redis: %w", err)
	}
	return nil
}

func (s *Store) Lookup(ctx context.Context, token string) (identity.Session, error) {
	key := sessionKey(token)
	fields, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return identity.Session{}, fmt.Errorf("leer sesión: %w", err)
	}
	if len(fields) == 0 {
		return identity.Session{}, identity.ErrNotFound
	}

	sessID, err := uuid.Parse(fields["session_id"])
	if err != nil {
		return identity.Session{}, identity.ErrNotFound
	}
	userID, err := uuid.Parse(fields["user_id"])
	if err != nil {
		return identity.Session{}, identity.ErrNotFound
	}
	expires := unixTime(fields["expires_at"])
	if !expires.IsZero() && time.Now().After(expires) {
		_ = s.Delete(ctx, token)
		return identity.Session{}, identity.ErrNotFound
	}

	// TTL deslizante: la actividad prolonga la sesión hasta el tope absoluto.
	if ttl, err := s.rdb.TTL(ctx, key).Result(); err == nil && ttl > 0 {
		s.rdb.HSet(ctx, key, "last_seen_at", time.Now().Unix())
	}

	return identity.Session{
		ID:         sessID,
		UserID:     userID,
		Role:       fields["role"],
		IP:         fields["ip"],
		UserAgent:  fields["user_agent"],
		CreatedAt:  unixTime(fields["created_at"]),
		LastSeenAt: unixTime(fields["last_seen_at"]),
		ExpiresAt:  expires,
	}, nil
}

func (s *Store) Delete(ctx context.Context, token string) error {
	key := sessionKey(token)
	userID, err := s.rdb.HGet(ctx, key, "user_id").Result()
	if err == nil {
		s.rdb.SRem(ctx, "user_sessions:"+userID, key)
	}
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("borrar sesión: %w", err)
	}
	return nil
}

func (s *Store) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	idx := userKey(userID)
	keys, err := s.rdb.SMembers(ctx, idx).Result()
	if err != nil {
		return fmt.Errorf("listar sesiones del usuario: %w", err)
	}
	if len(keys) > 0 {
		if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("borrar sesiones del usuario: %w", err)
		}
	}
	return s.rdb.Del(ctx, idx).Err()
}

func unixTime(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}
