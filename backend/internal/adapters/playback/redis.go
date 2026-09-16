// Package playback implementa media.Reproduccion sobre Redis.
//
// Es el mismo patrón que las sesiones (ADR-0006): el testigo es opaco, y lo que
// se guarda es su hash, de modo que un volcado de Redis no entrega credenciales
// utilizables. Vive fuera del proceso porque cualquier instancia de la API debe
// poder resolver un testigo emitido por otra (CE-01).
package playback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"mooc/backend/internal/modules/media"
	"mooc/backend/internal/platform/ids"
)

type Store struct{ rdb *redis.Client }

func New(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func clave(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "play:" + hex.EncodeToString(sum[:])
}

func (s *Store) Emitir(ctx context.Context, assetID uuid.UUID, ttl time.Duration) (string, error) {
	token, err := ids.Token()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, clave(token), assetID.String(), ttl).Err(); err != nil {
		return "", fmt.Errorf("emitir credencial de reproducción: %w", err)
	}
	return token, nil
}

func (s *Store) Resolver(ctx context.Context, token string) (uuid.UUID, error) {
	valor, err := s.rdb.Get(ctx, clave(token)).Result()
	if errors.Is(err, redis.Nil) {
		return uuid.Nil, media.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolver credencial de reproducción: %w", err)
	}
	id, err := uuid.Parse(valor)
	if err != nil {
		return uuid.Nil, media.ErrNotFound
	}
	return id, nil
}
