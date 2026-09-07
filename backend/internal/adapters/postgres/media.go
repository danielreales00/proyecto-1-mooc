package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mooc/backend/internal/modules/media"
	"mooc/backend/internal/platform/dbx"
)

type MediaStore struct{ pool *pgxpool.Pool }

func NewMediaStore(pool *pgxpool.Pool) *MediaStore { return &MediaStore{pool: pool} }

func (s *MediaStore) DB() dbx.DB { return s.pool }

func (s *MediaStore) WithinTx(ctx context.Context, fn func(context.Context, dbx.DB) error) error {
	return withinTx(ctx, s.pool, fn)
}

func nfm(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return media.ErrNotFound
	}
	return err
}

const colsAsset = `id, owner_id, kind, original_key, original_filename, declared_mime,
	detected_mime, size_bytes, declared_sha256, sha256, duration_seconds, width, height,
	status, last_error, created_at, updated_at`

func (s *MediaStore) InsertarAsset(ctx context.Context, db dbx.DB, a media.Asset) error {
	_, err := db.Exec(ctx, `
		INSERT INTO media.assets
		    (id, owner_id, kind, original_key, original_filename, declared_mime,
		     size_bytes, declared_sha256, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`,
		a.ID, a.OwnerID, a.Kind, a.OriginalKey, a.OriginalFilename, a.DeclaredMIME,
		a.SizeBytes, a.DeclaredSHA256, a.Status, a.CreatedAt)
	return err
}

func (s *MediaStore) AssetPorID(ctx context.Context, db dbx.DB, id uuid.UUID) (media.Asset, error) {
	var a media.Asset
	err := db.QueryRow(ctx, `SELECT `+colsAsset+` FROM media.assets WHERE id=$1`, id).
		Scan(&a.ID, &a.OwnerID, &a.Kind, &a.OriginalKey, &a.OriginalFilename, &a.DeclaredMIME,
			&a.DetectedMIME, &a.SizeBytes, &a.DeclaredSHA256, &a.SHA256, &a.DurationSeconds,
			&a.Width, &a.Height, &a.Status, &a.LastError, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return media.Asset{}, nfm(err)
	}
	return a, nil
}

func (s *MediaStore) ActualizarAsset(ctx context.Context, db dbx.DB, a media.Asset) error {
	_, err := db.Exec(ctx, `
		UPDATE media.assets
		   SET detected_mime=$2, sha256=$3, duration_seconds=$4, width=$5, height=$6,
		       status=$7, last_error=$8, updated_at=$9
		 WHERE id=$1`,
		a.ID, a.DetectedMIME, a.SHA256, a.DurationSeconds, a.Width, a.Height,
		a.Status, a.LastError, a.UpdatedAt)
	return err
}

func (s *MediaStore) InsertarCarga(ctx context.Context, db dbx.DB, c media.Carga) error {
	_, err := db.Exec(ctx, `
		INSERT INTO media.uploads (id, asset_id, s3_upload_id, part_size, total_parts, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.AssetID, c.S3UploadID, c.PartSize, c.TotalParts, c.ExpiresAt)
	return err
}

func (s *MediaStore) CargaDeAsset(ctx context.Context, db dbx.DB, assetID uuid.UUID) (media.Carga, error) {
	var c media.Carga
	err := db.QueryRow(ctx, `
		SELECT id, asset_id, s3_upload_id, part_size, total_parts, expires_at, completed_at, aborted_at
		  FROM media.uploads WHERE asset_id=$1`, assetID).
		Scan(&c.ID, &c.AssetID, &c.S3UploadID, &c.PartSize, &c.TotalParts,
			&c.ExpiresAt, &c.CompletedAt, &c.AbortedAt)
	if err != nil {
		return media.Carga{}, nfm(err)
	}
	return c, nil
}

func (s *MediaStore) CerrarCarga(ctx context.Context, db dbx.DB, assetID uuid.UUID, abortada bool) error {
	col := "completed_at"
	if abortada {
		col = "aborted_at"
	}
	_, err := db.Exec(ctx,
		`UPDATE media.uploads SET `+col+` = now() WHERE asset_id=$1`, assetID)
	return err
}
